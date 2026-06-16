package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolMongoDB = "builtin_mongodb"

var allowedMongoOps = map[string]bool{
	"list_dbs":         true,
	"list_collections": true,
	"find":             true,
	"count":            true,
	"explain":          true,
	"stats":            true,
	"indexes":          true,
}

func execBuiltinMongoDB(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_dbs"
	}

	if !allowedMongoOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedMongoOps)
	}

	connStr := strArg(in, "connection_string", "uri", "conn", "url")
	if connStr == "" {
		connStr = "mongodb://localhost:27017"
	}
	database := strArg(in, "database", "db", "db_name")
	collection := strArg(in, "collection", "coll", "col")
	filter := strArg(in, "filter", "query", "where")
	if filter == "" {
		filter = "{}"
	}
	limit := strArg(in, "limit", "max", "size")
	if limit == "" {
		limit = "20"
	}
	sort := strArg(in, "sort", "order", "sort_by")

	var script string

	switch op {
	case "list_dbs":
		script = fmt.Sprintf(`
var conn = connect("%s");
var dbs = conn.adminCommand({listDatabases: 1});
var names = dbs.databases.map(function(db) { return db.name + " (" + (db.sizeOnDisk / 1024 / 1024).toFixed(1) + " MB)"; });
print(names.join("\n"));
conn.close();
`, connStr)
	case "list_collections":
		if database == "" {
			return "", fmt.Errorf("database is required for list_collections")
		}
		script = fmt.Sprintf(`
var db = connect("%s/%s");
var cols = db.getCollectionNames();
print(cols.join("\n"));
`, connStr, database)
	case "find":
		if database == "" || collection == "" {
			return "", fmt.Errorf("database and collection are required for find")
		}
		script = fmt.Sprintf(`
var db = connect("%s/%s");
var docs = db.%s.find(%s).limit(%s).sort(%s).toArray();
print(JSON.stringify(docs, null, 2));
`, connStr, database, collection, filter, limit, sortOrDefault(sort))
	case "count":
		if database == "" || collection == "" {
			return "", fmt.Errorf("database and collection are required for count")
		}
		script = fmt.Sprintf(`
var db = connect("%s/%s");
var count = db.%s.countDocuments(%s);
print("Count: " + count);
`, connStr, database, collection, filter)
	case "explain":
		if database == "" || collection == "" {
			return "", fmt.Errorf("database and collection are required for explain")
		}
		script = fmt.Sprintf(`
var db = connect("%s/%s");
var exp = db.%s.find(%s).explain("executionStats");
print(JSON.stringify(exp, null, 2));
`, connStr, database, collection, filter)
	case "stats":
		if database == "" || collection == "" {
			return "", fmt.Errorf("database and collection are required for stats")
		}
		script = fmt.Sprintf(`
var db = connect("%s/%s");
var stats = db.%s.stats();
print(JSON.stringify(stats, null, 2));
`, connStr, database, collection)
	case "indexes":
		if database == "" || collection == "" {
			return "", fmt.Errorf("database and collection are required for indexes")
		}
		script = fmt.Sprintf(`
var db = connect("%s/%s");
var idxs = db.%s.getIndexes();
print(JSON.stringify(idxs, null, 2));
`, connStr, database, collection)
	}

	cmd := exec.Command("mongosh", "--quiet", "--eval", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("mongodb %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("mongodb %s: (no output)", op), nil
	}
	return fmt.Sprintf("mongodb %s result:\n\n%s", op, result), nil
}

func sortOrDefault(sort string) string {
	if sort == "" {
		return "{}"
	}
	return sort
}

func NewBuiltinMongoDBTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolMongoDB,
			Desc:  "Read-only MongoDB operations: list databases, list collections, find documents, count, explain query, stats, indexes.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":        {Type: einoschema.String, Desc: "Operation: list_dbs, list_collections, find, count, explain, stats, indexes", Required: false},
				"connection_string": {Type: einoschema.String, Desc: "MongoDB connection string (default: mongodb://localhost:27017)", Required: false},
				"database":         {Type: einoschema.String, Desc: "Database name", Required: false},
				"collection":       {Type: einoschema.String, Desc: "Collection name", Required: false},
				"filter":           {Type: einoschema.String, Desc: "JSON filter document (default: {})", Required: false},
				"limit":            {Type: einoschema.String, Desc: "Max documents to return (default: 20)", Required: false},
				"sort":             {Type: einoschema.String, Desc: "JSON sort document (default: {})", Required: false},
			}),
		},
		execBuiltinMongoDB,
	)
}
