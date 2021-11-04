package connection

import (
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/spf13/viper"
)

func Create() *azuredevops.Connection {
	baseAddress := viper.GetString("tfsBaseAddress")

	personalAccessToken := viper.GetString("tfsAccessToken")
	if personalAccessToken != "" {
		return azuredevops.NewPatConnection(baseAddress, personalAccessToken)
	}

	return azuredevops.NewAnonymousConnection(baseAddress)
}
