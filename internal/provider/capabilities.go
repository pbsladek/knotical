package provider

type Transport string

const (
	TransportAPI Transport = "api"
	TransportCLI Transport = "cli"
)

type Capabilities struct {
	NativeStreaming    bool
	NativeConversation bool
	NativeSchema       bool
	ModelListing       bool
}

func NormalizeTransport(value string) Transport {
	if value == string(TransportCLI) {
		return TransportCLI
	}
	return TransportAPI
}

func CapabilitiesForTransport(name string, transport string) Capabilities {
	switch NormalizeTransport(transport) {
	case TransportCLI:
		return Capabilities{
			NativeStreaming:    false,
			NativeConversation: false,
			NativeSchema:       false,
			ModelListing:       false,
		}
	default:
		switch name {
		case "openai", "anthropic", "gemini", "ollama":
			return Capabilities{
				NativeStreaming:    true,
				NativeConversation: true,
				NativeSchema:       name != "ollama",
				ModelListing:       true,
			}
		default:
			return Capabilities{}
		}
	}
}
