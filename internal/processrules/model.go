package processrules

type Identity struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type Status struct {
	Type    string   `json:"type"`
	Value   string   `json:"value"`
	State   string   `json:"state"`
	Matches int      `json:"matches"`
	Paths   []string `json:"paths,omitempty"`
	Message string   `json:"message"`
}
