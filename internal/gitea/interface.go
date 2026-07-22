package gitea

// ClientInterface defines the Gitea operations needed by the API handlers.
type ClientInterface interface {
	AdminToken() string
	CreateGiteaUser(username, password string) error
	AddUserToOrg(username string) error
	CreateUserRepo(username string) (*CreateRepoResponse, error)
	CreateUserToken(username, password string) (*CreateTokenResponse, error)
	PublicCloneURL(username string) string
}
