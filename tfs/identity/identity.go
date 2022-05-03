package identity

import (
	"context"
	"errors"
	"tasker/ptr"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v6"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v6/identity"
	"github.com/spf13/viper"
)

type Identity struct {
	Id          string
	DisplayName string
}

func Get(ctx context.Context, conn *azuredevops.Connection) (*Identity, error) {
	userFilter := viper.GetString("tfsUserFilter")

	client, err := identity.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	identities, err := client.ReadIdentities(ctx, identity.ReadIdentitiesArgs{
		SearchFilter:    ptr.FromStr("General"),
		FilterValue:     ptr.FromStr(userFilter),
		QueryMembership: &identity.QueryMembershipValues.None,
	})
	if err != nil {
		return nil, err
	}

	if identities == nil || len(*identities) == 0 {
		return nil, errors.New("user identity not found")
	}

	if len(*identities) > 1 {
		return nil, errors.New("user filter not unique")
	}

	identity := (*identities)[0]

	return &Identity{
		Id:          identity.Id.String(),
		DisplayName: *identity.ProviderDisplayName,
	}, nil
}
