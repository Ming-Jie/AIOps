package imhistory

var globalRecorder *Recorder

// SetGlobalRecorder wires the shared recorder used by IM bot clients (lark, telegram).
func SetGlobalRecorder(r *Recorder) {
	globalRecorder = r
}

// GlobalRecorder returns the recorder set at server boot.
func GlobalRecorder() *Recorder {
	return globalRecorder
}
