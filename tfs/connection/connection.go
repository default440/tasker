package connection

import (
	"crypto/tls"
	"net/http"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/spf13/viper"
)

func Create() *azuredevops.Connection {
	baseAddress := viper.GetString("tfsBaseAddress")

	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultTransport = customTransport

	personalAccessToken := viper.GetString("tfsAccessToken")
	if personalAccessToken != "" {
		return azuredevops.NewPatConnection(baseAddress, personalAccessToken)
	}

	return azuredevops.NewAnonymousConnection(baseAddress)
}
