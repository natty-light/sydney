package messages

type Method string

const (
	Initialize Method = "initialize"
)

const Version = "2.0"

type Request struct {
	Version string      `json:"jsonrpc"`
	Method  Method      `json:"method"`
	Params  interface{} `json:"params"`
	Id      interface{} `json:"id"`
}

type Response struct {
	Version string      `json:"jsonrpc"`
	Result  Result      `json:"result"`
	Error   interface{} `json:"error"`
	Id      interface{} `json:"id"`
}

type Result interface {
	Result()
}
