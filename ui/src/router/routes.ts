import { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  { path: '/login', component: () => import('layouts/LoginPage.vue') },
  { path: '/register', component: () => import('layouts/RegisterPage.vue') },
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: '/dashboard', name: 'dashboard', component: () => import('pages/DashboardPage.vue'), meta: { requireAuth: true, title: '仪表盘' } },
      { path: '/chat/:agentId?', name: 'chat', component: () => import('pages/ChatPage.vue'), meta: { requireAuth: true, title: '对话' } },
      { path: '/agents', name: 'agents', component: () => import('pages/AgentsPage.vue'), meta: { requireAuth: true, title: '智能体' } },
      { path: '/skills', name: 'skills', component: () => import('pages/SkillsPage.vue'), meta: { requireAuth: true, title: '技能' } },
      { path: '/mcp', name: 'mcp', component: () => import('pages/MCPPage.vue'), meta: { requireAuth: true, title: 'MCP' } },
      { path: '/models', name: 'models', component: () => import('pages/ModelsPage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '模型配置' } },
      { path: '/workflows', name: 'workflows', component: () => import('pages/WorkflowsPage.vue'), meta: { requireAuth: true, title: '工作流' } },
      { path: '/schedules', name: 'schedules', component: () => import('pages/SchedulesPage.vue'), meta: { requireAuth: true, title: '定时任务' } },
      { path: '/channels', name: 'channels', component: () => import('pages/ChannelsPage.vue'), meta: { requireAuth: true, title: '消息通知' } },
      { path: '/guardrails', name: 'guardrails', component: () => import('pages/GuardrailsPage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '安全护栏' } },
      { path: '/teams', name: 'teams', component: () => import('pages/TeamsPage.vue'), meta: { requireAuth: true, title: '智能体团队' } },
      { path: '/teams/:teamId/chat', name: 'team-chat', component: () => import('pages/TeamChatPage.vue'), meta: { requireAuth: true, title: '团队对话' } },
      { path: '/eval', name: 'eval', component: () => import('pages/EvaluationPage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '智能体评测' } },
      { path: '/knowledge', name: 'knowledge', component: () => import('pages/KnowledgeBasePage.vue'), meta: { requireAuth: true, title: '知识库' } },
      { path: '/knowledge/:id', name: 'knowledge-detail', component: () => import('pages/KnowledgeBaseDetailPage.vue'), meta: { requireAuth: true, title: '知识库详情' } },
      { path: '/larkbots', redirect: '/bots' },
      { path: '/bots', name: 'bots', component: () => import('pages/BotPage.vue'), meta: { requireAuth: true, title: '智能机器人' } },
      { path: '/audit-logs', name: 'audit-logs', component: () => import('pages/AuditLogsPage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '审计日志' } },
      { path: '/approvals', name: 'approvals', component: () => import('pages/ApprovalsPage.vue'), meta: { requireAuth: true, title: '审批管理' } },
      { path: '/roles', name: 'roles', component: () => import('pages/RBACPage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '角色管理' } },
      { path: '/users', name: 'users', component: () => import('pages/UsersPage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '用户' } },
      { path: '/usage', name: 'usage', component: () => import('pages/UsagePage.vue'), meta: { requireAuth: true, requiresAdmin: true, title: '用量统计' } },
      { path: '/rbac', redirect: { name: 'roles' } },
      { path: '/permissions', redirect: { name: 'roles' } },
      { path: '/user-roles', redirect: { name: 'users' } },
      { path: '/profile', name: 'profile', component: () => import('pages/ProfilePage.vue'), meta: { requireAuth: true, title: '个人资料' } }
    ]
  },
  { path: '/:catchAll(.*)*', component: () => import('pages/ErrorNotFound.vue') }
]

export default routes
