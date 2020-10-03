package taigo

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Client is the session manager of Taiga Driver
type Client struct {
	Credentials        *Credentials
	APIURL             string       // set by system
	APIversion         string       // default: "v1"
	BaseURL            string       // i.e.: "http://taiga.test" | Same value as `api` in `taiga-front-dist/dist/conf.json`
	Headers            *http.Header // mostly set by system
	HTTPClient         *http.Client // set by user
	Token              string       // set by system; can be set manually
	TokenType          string       // default=Bearer; options:Bearer,Application
	Self               *User        // User logged in
	defaultProjectID   int          // Project used in query params by default
	pagination         *Pagination  // Pagination details extracted from the LAST http response
	paginationDisabled bool         // indicates pagination status
	isInitialised      bool         // indicates if taiga.Client has been initialised already

	// Core Services
	Request *RequestService

	// Taiga Services
	Auth      *AuthService
	Epic      *EpicService
	Issue     *IssueService
	Milestone *MilestoneService
	Project   *ProjectService
	Resolver  *ResolverService
	Stats     *StatsService
	Task      *TaskService
	UserStory *UserStoryService
	User      *UserService
	Webhook   *WebhookService
	Wiki      *WikiService
}

// MakeURL accepts an Endpoint URL and returns a compiled absolute URL
//
// For example:
//	* If the given endpoint URLs are [epics, attachments]
//	* If the BaseURL is https://api.taiga.io
//	* It returns https://api.taiga.io/api/v1/epics/attachments
//  * Suffixes are appended to the URL joined by a slash (/)
func (c *Client) MakeURL(EndpointParts ...string) string {
	return c.APIURL + "/" + strings.Join(EndpointParts, "/")
}

// SetDefaultProject takes an int and uses that to internally set the default project ID.
func (c *Client) SetDefaultProject(projectID int) error {
	if !(projectID > 0) {
		return fmt.Errorf("Could not set Default Project ID. Provided projectID was: %d", projectID)
	}
	c.defaultProjectID = projectID
	return nil
}

// SetDefaultProjectBySlug takes an slug string and uses that to internally set the default project ID.
func (c *Client) SetDefaultProjectBySlug(projectSlug string) error {
	if projectSlug == "" {
		return fmt.Errorf("Could not set Default Project ID. Provided projectSlug was: %s", projectSlug)
	}
	proj, err := c.Project.GetBySlug(projectSlug)
	if err != nil {
		return err
	}
	c.defaultProjectID = proj.ID
	return nil
}

// GetDefaultProject returns the currently set Default Project ID
func (c *Client) GetDefaultProject() int {
	return c.defaultProjectID
}

// GetDefaultProjectAsQueryParam returns the currently set Default Project ID formatted as a QueryParam
func (c *Client) GetDefaultProjectAsQueryParam() string {
	return "?project=" + strconv.Itoa(c.defaultProjectID)
}

// ClearDefaultProject resets the currently set Default Project ID to 0 (None)
func (c *Client) ClearDefaultProject() {
	c.defaultProjectID = 0
}

// HasDefaultProject returns true if there's a Default Project ID set
func (c *Client) HasDefaultProject() bool {
	if c.defaultProjectID > 0 {
		return true
	}
	return false
}

// Initialise returns a new Taiga Client which is the entrypoint of the driver
// Initialise() is automatically called by the `AuthByCredentials` and `AuthByToken` methods.
// If you, for some reason, would like to manually set the Client.Token field, then Initialise() must be called manually!
func (c *Client) Initialise() error {
	// Skip if already Initialised
	if c.isInitialised {
		return nil
	}
	// Taiga.Client safety guards
	if len(c.BaseURL) < len("http://") { // compares for a minimum of len("http://")
		return fmt.Errorf("BaseURL is not set or invalid")

	}
	//Set basic token type
	if len(c.TokenType) <= 1 {
		c.TokenType = "Bearer"
	}
	// Set basic headers
	c.Headers = &http.Header{}
	c.setContentTypeToJSON() // Default Header = {"Content-Type": "application/json"}

	// Setup APIversion | If user did not set it to anything, default to `v1`
	if c.APIversion == "" {
		c.APIversion = "v1"
	}
	// Compile URL to Taiga API | Example: https://api.taiga.io/api/v1
	c.APIURL = c.BaseURL + "/api/" + c.APIversion

	// Disable pagination ()
	c.DisablePagination(true) // https://taigaio.github.io/taiga-doc/dist/api.html#_pagination

	// Bootstrapping Services
	c.Request = &RequestService{c}

	c.Auth = &AuthService{c, "auth"}
	c.Epic = &EpicService{c, "epics"}
	c.Issue = &IssueService{c, "issues"}
	c.Milestone = &MilestoneService{c, "milestones"}
	c.Project = &ProjectService{c, "projects"}
	c.Resolver = &ResolverService{c, "resolver"}
	c.Stats = &StatsService{c, "stats"}
	c.Task = &TaskService{c, "tasks"}
	c.UserStory = &UserStoryService{c, "userstories"}
	c.User = &UserService{c, "users"}
	c.Webhook = &WebhookService{c, "webhooks", "webhooklogs"}
	c.Wiki = &WikiService{c, "wiki"}

	// Final steps
	c.isInitialised = true
	return nil
}

// AuthByCredentials authenticates to Taiga using the provided basic credentials
func (c *Client) AuthByCredentials(credentials *Credentials) error {
	if err := c.Initialise(); err != nil {
		return err
	}

	if len(credentials.Type) <= 1 {
		return fmt.Errorf("LoginType is not set")
	}

	user, err := c.Auth.login(credentials)
	if err != nil {
		return err
	}

	c.Self = user.AsUser()
	return nil
}

// AuthByToken authenticates to Taiga using provided Token by requesting users/me
func (c *Client) AuthByToken(tokenType, token string) error {
	if err := c.Initialise(); err != nil {
		return err
	}
	c.TokenType = tokenType
	c.Token = token

	var err error
	c.Self, err = c.User.Me()
	if err != nil {
		return fmt.Errorf("Authentication has failed. Reason: %s", err)
	}
	return nil
}

// DisablePagination controls the value of header `x-disable-pagination`.
func (c *Client) DisablePagination(b bool) {
	var decision string = strings.Title(strconv.FormatBool(b))
	m := map[string]string{
		"x-disable-pagination": decision,
	}
	c.LoadExternalHeaders(m)
	c.paginationDisabled = b
}

// GetPagination returns the Pagination struct created from the last response
func (c *Client) GetPagination() Pagination {
	return *c.pagination
}

// GetAuthorizationHeader returns the formatted value of Authorization key from Headers
func (c *Client) GetAuthorizationHeader() string {
	return c.Headers.Get("Authorization")
}

// LoadExternalHeaders loads a map of header key/value pairs permemently into `Client.Headers`
func (c *Client) LoadExternalHeaders(headers map[string]string) {
	for k, v := range headers {
		c.Headers.Add(k, v)
	}
}

func (c *Client) setContentTypeToJSON() {
	c.Headers.Add("Content-Type", "application/json")
}

func (c *Client) setToken() {
	c.Headers.Add("Authorization", c.TokenType+" "+c.Token)
}

// loadHeaders takes an http.Request and maps locally stored Header values to its .Header
func (c *Client) loadHeaders(request *http.Request) {
	for key, values := range *c.Headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
}
