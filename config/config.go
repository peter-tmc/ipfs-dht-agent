package config

type Configuration struct {
	Keys []string
	Agent AgentConfiguration
}



type AgentConfiguration struct {
	NoAgents int
	AgentN int
}