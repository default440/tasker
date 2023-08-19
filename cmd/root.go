package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tasker",
	Short: "TFS task creator",
	Long:  `This application is a tool to rapidly create TFS tasks and synchronize them with wiki.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().Bool("debug", false, "The Debug Mode")
	cobra.CheckErr(viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("tfsBaseAddress", "https://msk-tfs-t.infotecs-nt/tfs/SrvNccCollection")
	viper.SetDefault("tfsProject", "NSMS")
	viper.SetDefault("tfsTeam", "SMP")
	viper.SetDefault("tfsDiscipline", "Development")
	viper.SetDefault("tfsUserFilter", "ANON")
	viper.SetDefault("tfsAccessToken", "")
	viper.SetDefault("tfsBugfixUserStoryNamePattern", "")
	viper.SetDefault("tfsCommonUserStoryNamePattern", "")
	viper.SetDefault("tfsBugTitleTemplate", defaultBugTitleTemplate)
	viper.SetDefault("wikiAccessToken", "")
	viper.SetDefault("wikiBaseAddress", "https://wiki.infotecs.int")

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(".")
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".tasker")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("%v; config: %s\n", err, viper.ConfigFileUsed())
	}
}
