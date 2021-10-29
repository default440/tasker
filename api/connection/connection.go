package connection

import (
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/spf13/viper"
)

func Create() *azuredevops.Connection {
	baseAddress := viper.GetString("baseAddress")

	personalAccessToken := viper.GetString("personalAccessToken")
	if personalAccessToken != "" {
		return azuredevops.NewPatConnection(baseAddress, personalAccessToken)
	}

	return azuredevops.NewAnonymousConnection(baseAddress)
}
