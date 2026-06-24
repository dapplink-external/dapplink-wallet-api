package flags

import "github.com/urfave/cli/v2"

const envVarPrefix = "WALLET_API"

func prefixEnvVars(name string) []string {
	return []string{envVarPrefix + "_" + name}
}

var (
	ConfigPathFlag = &cli.StringFlag{
		Name:    "yaml-config",
		Usage:   "The path of the yaml config file",
		EnvVars: prefixEnvVars("YAML_CONFIG"),
		Aliases: []string{"c"},
		Value:   "./config.yaml",
	}
)

var requireFlags = []cli.Flag{
	ConfigPathFlag,
}

var optionalFlags = []cli.Flag{}

var Flags []cli.Flag

func init() {
	Flags = append(requireFlags, optionalFlags...)
}
