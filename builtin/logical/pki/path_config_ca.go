package pki

import (
	"context"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func pathConfigCA(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/ca",
		Fields: map[string]*framework.FieldSchema{
			"pem_bundle": {
				Type: framework.TypeString,
				Description: `PEM-format, concatenated unencrypted
secret key and certificate.`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathImportIssuers,
				// Read more about why these flags are set in backend.go.
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathConfigCAHelpSyn,
		HelpDescription: pathConfigCAHelpDesc,
	}
}

const pathConfigCAHelpSyn = `
Set the CA certificate and private key used for generated credentials.
`

const pathConfigCAHelpDesc = `
This sets the CA information used for credentials generated by this
by this mount. This must be a PEM-format, concatenated unencrypted
secret key and certificate.

For security reasons, the secret key cannot be retrieved later.
`

func pathConfigIssuers(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/issuers",
		Fields: map[string]*framework.FieldSchema{
			defaultRef: {
				Type:        framework.TypeString,
				Description: `Reference (name or identifier) to the default issuer.`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCAIssuersRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathCAIssuersWrite,
				// Read more about why these flags are set in backend.go.
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathConfigIssuersHelpSyn,
		HelpDescription: pathConfigIssuersHelpDesc,
	}
}

func pathReplaceRoot(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "root/replace",
		Fields: map[string]*framework.FieldSchema{
			"default": {
				Type:        framework.TypeString,
				Description: `Reference (name or identifier) to the default issuer.`,
				Default:     "next",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathCAIssuersWrite,
				// Read more about why these flags are set in backend.go.
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
		},

		HelpSynopsis:    pathConfigIssuersHelpSyn,
		HelpDescription: pathConfigIssuersHelpDesc,
	}
}

func (b *backend) pathCAIssuersRead(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot read defaults until migration has completed"), nil
	}

	config, err := getIssuersConfig(ctx, req.Storage)
	if err != nil {
		return logical.ErrorResponse("Error loading issuers configuration: " + err.Error()), nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			defaultRef: config.DefaultIssuerId,
		},
	}, nil
}

func (b *backend) pathCAIssuersWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Since we're planning on updating issuers here, grab the lock so we've
	// got a consistent view.
	b.issuersLock.Lock()
	defer b.issuersLock.Unlock()

	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot update defaults until migration has completed"), nil
	}

	newDefault := data.Get(defaultRef).(string)
	if len(newDefault) == 0 || newDefault == defaultRef {
		return logical.ErrorResponse("Invalid issuer specification; must be non-empty and can't be 'default'."), nil
	}

	parsedIssuer, err := resolveIssuerReference(ctx, req.Storage, newDefault)
	if err != nil {
		return logical.ErrorResponse("Error resolving issuer reference: " + err.Error()), nil
	}

	response := &logical.Response{
		Data: map[string]interface{}{
			"default": parsedIssuer,
		},
	}

	entry, err := fetchIssuerById(ctx, req.Storage, parsedIssuer)
	if err != nil {
		return logical.ErrorResponse("Unable to fetch issuer: " + err.Error()), nil
	}

	if len(entry.KeyID) == 0 {
		msg := "This selected default issuer has no key associated with it. Some operations like issuing certificates and signing CRLs will be unavailable with the requested default issuer until a key is imported or the default issuer is changed."
		response.AddWarning(msg)
		b.Logger().Error(msg)
	}

	err = updateDefaultIssuerId(ctx, req.Storage, parsedIssuer)
	if err != nil {
		return logical.ErrorResponse("Error updating issuer configuration: " + err.Error()), nil
	}

	return response, nil
}

const pathConfigIssuersHelpSyn = `Read and set the default issuer certificate for signing.`

const pathConfigIssuersHelpDesc = `
This path allows configuration of issuer parameters.

Presently, the "default" parameter controls which issuer is the default,
accessible by the existing signing paths (/root/sign-intermediate,
/root/sign-self-issued, /sign-verbatim, /sign/:role, and /issue/:role).

The /root/replace path is aliased to this path, with default taking the
value of the issuer with the name "next", if it exists.
`

func pathConfigKeys(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: "config/keys",
		Fields: map[string]*framework.FieldSchema{
			defaultRef: {
				Type:        framework.TypeString,
				Description: `Reference (name or identifier) of the default key.`,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback:                    b.pathKeyDefaultWrite,
				ForwardPerformanceStandby:   true,
				ForwardPerformanceSecondary: true,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback:                    b.pathKeyDefaultRead,
				ForwardPerformanceStandby:   false,
				ForwardPerformanceSecondary: false,
			},
		},

		HelpSynopsis:    pathConfigKeysHelpSyn,
		HelpDescription: pathConfigKeysHelpDesc,
	}
}

func (b *backend) pathKeyDefaultRead(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot read key defaults until migration has completed"), nil
	}

	config, err := getKeysConfig(ctx, req.Storage)
	if err != nil {
		return logical.ErrorResponse("Error loading keys configuration: " + err.Error()), nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			defaultRef: config.DefaultKeyId,
		},
	}, nil
}

func (b *backend) pathKeyDefaultWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	// Since we're planning on updating keys here, grab the lock so we've
	// got a consistent view.
	b.issuersLock.Lock()
	defer b.issuersLock.Unlock()

	if b.useLegacyBundleCaStorage() {
		return logical.ErrorResponse("Cannot update key defaults until migration has completed"), nil
	}

	newDefault := data.Get(defaultRef).(string)
	if len(newDefault) == 0 || newDefault == defaultRef {
		return logical.ErrorResponse("Invalid key specification; must be non-empty and can't be 'default'."), nil
	}

	parsedKey, err := resolveKeyReference(ctx, req.Storage, newDefault)
	if err != nil {
		return logical.ErrorResponse("Error resolving issuer reference: " + err.Error()), nil
	}

	err = updateDefaultKeyId(ctx, req.Storage, parsedKey)
	if err != nil {
		return logical.ErrorResponse("Error updating issuer configuration: " + err.Error()), nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			defaultRef: parsedKey,
		},
	}, nil
}

const pathConfigKeysHelpSyn = `Read and set the default key used for signing`

const pathConfigKeysHelpDesc = `
This path allows configuration of key parameters.

The "default" parameter controls which key is the default used by signing paths.
`
