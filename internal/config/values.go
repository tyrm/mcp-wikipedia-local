package config

// Values contains the type of each value.
type Values struct {
	LogLevel        string
	ConfigPath      string
	SoftwareVersion string
}

// Defaults contains the default values.
var Defaults = Values{
	ConfigPath:      "",
	LogLevel:        "info",
	SoftwareVersion: "dev",
}
