package control

type Request struct {
	Command string `json:"command"`
	Mode    string `json:"mode,omitempty"`
}
type Response struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func valid(r Request) bool {
	switch r.Command {
	case "status", "diagnostics", "validate", "reload", "rules-update":
		return r.Mode == ""
	case "set-mode":
		switch r.Mode {
		case "auto", "prefer-de", "prefer-nl", "direct":
			return true
		}
	}
	return false
}
