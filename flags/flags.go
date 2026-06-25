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
	SponsorPrivateKeyFlag = &cli.StringFlag{
		Name:    "sponsor-private-key",
		Usage:   "Sponsor EOA private key for AA SetCode transactions (hex, with or without 0x prefix)",
		EnvVars: append([]string{"SPONSOR_KEY"}, prefixEnvVars("SPONSOR_KEY")...),
	}
	PaymasterSignerKeyFlag = &cli.StringFlag{
		Name:    "paymaster-signer-key",
		Usage:   "Paymaster verifying signer private key for off-chain UserOp authorization (hex, with or without 0x prefix)",
		EnvVars: append([]string{"PAYMASTER_SIGNER_KEY"}, prefixEnvVars("PAYMASTER_SIGNER_KEY")...),
	}
)

var requireFlags = []cli.Flag{
	ConfigPathFlag,
}

var optionalFlags = []cli.Flag{
	SponsorPrivateKeyFlag,
	PaymasterSignerKeyFlag,
}

var Flags []cli.Flag

func init() {
	Flags = append(requireFlags, optionalFlags...)
}
