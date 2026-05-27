package ldap

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
	"github.com/raychao-oao/ad-mcp/internal/config"
)

// Client wraps an LDAP connection with AD-specific helpers.
type Client struct {
	conn   *goldap.Conn
	cfg    config.LDAPConfig
	baseDN string
}

func New(cfg config.LDAPConfig) (*Client, error) {
	c := &Client{cfg: cfg, baseDN: cfg.BaseDN}
	if err := c.dial(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) dial() error {
	if c.conn != nil {
		c.conn.Close()
	}
	conn, err := goldap.DialURL(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("ldap dial: %w", err)
	}
	if err := conn.Bind(c.cfg.BindDN, c.cfg.Password); err != nil {
		conn.Close()
		return fmt.Errorf("ldap bind: %w", err)
	}
	c.conn = conn
	return nil
}

func (c *Client) Close() { c.conn.Close() }

// User holds the fields we extract from an AD user entry.
type User struct {
	DN                string
	SamAccountName    string
	DisplayName       string
	Email             string
	Department        string
	Title             string
	ManagerDN         string
	OU                string
	Disabled          bool
	Locked            bool
	PasswordNeverExp  bool
	MustChangePwd     bool
	BadPwdCount       int
	AccountExpires    *time.Time // nil = never
	LastLogon         *time.Time
	PwdLastSet        *time.Time
}

// Group holds fields from an AD group entry.
type Group struct {
	DN          string
	Name        string
	Description string
	OU          string
	MemberCount int
}

var userAttrs = []string{
	"distinguishedName", "sAMAccountName", "displayName", "mail",
	"department", "title", "manager", "userAccountControl",
	"lockoutTime", "badPwdCount", "accountExpires", "lastLogonTimestamp",
	"pwdLastSet",
}

var groupAttrs = []string{
	"distinguishedName", "cn", "description", "member",
}

// SearchUsers searches AD users matching the query string (name, email, or sAMAccountName).
func (c *Client) SearchUsers(query string) ([]User, error) {
	escaped := goldap.EscapeFilter(query)
	filter := fmt.Sprintf(
		"(&(objectClass=user)(objectCategory=person)(|(sAMAccountName=*%s*)(displayName=*%s*)(mail=*%s*)))",
		escaped, escaped, escaped,
	)
	return c.searchUsers(filter)
}

// GetUser fetches a single user by sAMAccountName or DN.
func (c *Client) GetUser(id string) (*User, error) {
	var filter string
	if strings.Contains(id, "=") {
		// looks like a DN
		filter = fmt.Sprintf("(&(objectClass=user)(distinguishedName=%s))", goldap.EscapeFilter(id))
	} else {
		filter = fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", goldap.EscapeFilter(id))
	}
	users, err := c.searchUsers(filter)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	return &users[0], nil
}

// GetUserGroups returns the groups a user belongs to.
func (c *Client) GetUserGroups(userDN string) ([]Group, error) {
	filter := fmt.Sprintf("(&(objectClass=group)(member=%s))", goldap.EscapeFilter(userDN))
	return c.searchGroups(filter)
}

// SearchGroups searches groups by name or description.
func (c *Client) SearchGroups(query string) ([]Group, error) {
	escaped := goldap.EscapeFilter(query)
	filter := fmt.Sprintf(
		"(&(objectClass=group)(|(cn=*%s*)(description=*%s*)))",
		escaped, escaped,
	)
	return c.searchGroups(filter)
}

// GetGroup fetches a single group by cn or DN.
func (c *Client) GetGroup(id string) (*Group, error) {
	var filter string
	if strings.Contains(id, "=") {
		filter = fmt.Sprintf("(&(objectClass=group)(distinguishedName=%s))", goldap.EscapeFilter(id))
	} else {
		filter = fmt.Sprintf("(&(objectClass=group)(cn=%s))", goldap.EscapeFilter(id))
	}
	groups, err := c.searchGroups(filter)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("group not found: %s", id)
	}
	return &groups[0], nil
}

// ListGroupMembers returns users who are direct members of a group.
func (c *Client) ListGroupMembers(groupDN string) ([]User, error) {
	filter := fmt.Sprintf(
		"(&(objectClass=user)(objectCategory=person)(memberOf=%s))",
		goldap.EscapeFilter(groupDN),
	)
	return c.searchUsers(filter)
}

// FindLockedUsers returns all currently locked-out accounts.
func (c *Client) FindLockedUsers() ([]User, error) {
	// lockoutTime != 0 and lockoutTime > (now - lockout duration).
	// Simplest safe filter: lockoutTime >= 1 (any non-zero value).
	return c.searchUsers("(&(objectClass=user)(objectCategory=person)(lockoutTime>=1))")
}

// FindInactiveUsers returns accounts with no logon in the past `days` days.
func (c *Client) FindInactiveUsers(days int) ([]User, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	ft := toWindowsFileTime(cutoff)
	filter := fmt.Sprintf(
		"(&(objectClass=user)(objectCategory=person)(lastLogonTimestamp<=%d)(!(lastLogonTimestamp=0)))",
		ft,
	)
	return c.searchUsers(filter)
}

// FindDisabledUsers returns all disabled accounts.
func (c *Client) FindDisabledUsers() ([]User, error) {
	// userAccountControl bit 0x0002 = ACCOUNTDISABLE
	// Filter: userAccountControl:1.2.840.113556.1.4.803:=2
	return c.searchUsers("(&(objectClass=user)(objectCategory=person)(userAccountControl:1.2.840.113556.1.4.803:=2))")
}

// FindExpiringAccounts returns accounts expiring within the next `days` days.
func (c *Client) FindExpiringAccounts(days int) ([]User, error) {
	now := toWindowsFileTime(time.Now())
	future := toWindowsFileTime(time.Now().AddDate(0, 0, days))
	// accountExpires between now and future (exclusive of 0 and max = never)
	filter := fmt.Sprintf(
		"(&(objectClass=user)(objectCategory=person)(accountExpires>=%d)(accountExpires<=%d))",
		now, future,
	)
	return c.searchUsers(filter)
}

// FindPasswordNeverExpires returns accounts with DONT_EXPIRE_PASSWORD flag.
func (c *Client) FindPasswordNeverExpires() ([]User, error) {
	return c.searchUsers("(&(objectClass=user)(objectCategory=person)(userAccountControl:1.2.840.113556.1.4.803:=65536))")
}

// --- internal helpers ---

func (c *Client) searchUsers(filter string) ([]User, error) {
	req := goldap.NewSearchRequest(
		c.baseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases,
		0, 0, false, filter, userAttrs, nil,
	)
	res, err := c.conn.Search(req)
	if err != nil && isConnErr(err) {
		if rerr := c.dial(); rerr != nil {
			return nil, fmt.Errorf("ldap reconnect: %w", rerr)
		}
		res, err = c.conn.Search(req)
	}
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}
	users := make([]User, 0, len(res.Entries))
	for _, e := range res.Entries {
		users = append(users, entryToUser(e))
	}
	return users, nil
}

func (c *Client) searchGroups(filter string) ([]Group, error) {
	req := goldap.NewSearchRequest(
		c.baseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases,
		0, 0, false, filter, groupAttrs, nil,
	)
	res, err := c.conn.Search(req)
	if err != nil && isConnErr(err) {
		if rerr := c.dial(); rerr != nil {
			return nil, fmt.Errorf("ldap reconnect: %w", rerr)
		}
		res, err = c.conn.Search(req)
	}
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}
	groups := make([]Group, 0, len(res.Entries))
	for _, e := range res.Entries {
		groups = append(groups, entryToGroup(e))
	}
	return groups, nil
}

// isConnErr reports whether the error looks like a stale/dropped connection.
func isConnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "use of closed network connection") ||
		goldap.IsErrorWithCode(err, goldap.ErrorNetwork)
}

func entryToUser(e *goldap.Entry) User {
	uac, _ := strconv.ParseInt(e.GetAttributeValue("userAccountControl"), 10, 64)
	lockoutTime, _ := strconv.ParseInt(e.GetAttributeValue("lockoutTime"), 10, 64)
	badPwd, _ := strconv.Atoi(e.GetAttributeValue("badPwdCount"))

	u := User{
		DN:               e.DN,
		SamAccountName:   e.GetAttributeValue("sAMAccountName"),
		DisplayName:      e.GetAttributeValue("displayName"),
		Email:            e.GetAttributeValue("mail"),
		Department:       e.GetAttributeValue("department"),
		Title:            e.GetAttributeValue("title"),
		ManagerDN:        e.GetAttributeValue("manager"),
		OU:               ouFromDN(e.DN),
		Disabled:         uac&0x0002 != 0,
		Locked:           lockoutTime != 0,
		PasswordNeverExp: uac&0x10000 != 0,
		MustChangePwd:    e.GetAttributeValue("pwdLastSet") == "0",
		BadPwdCount:      badPwd,
	}

	if t := parseWinFileTime(e.GetAttributeValue("accountExpires")); t != nil {
		u.AccountExpires = t
	}
	if t := parseWinFileTime(e.GetAttributeValue("lastLogonTimestamp")); t != nil {
		u.LastLogon = t
	}
	if t := parseWinFileTime(e.GetAttributeValue("pwdLastSet")); t != nil {
		u.PwdLastSet = t
	}
	return u
}

func entryToGroup(e *goldap.Entry) Group {
	return Group{
		DN:          e.DN,
		Name:        e.GetAttributeValue("cn"),
		Description: e.GetAttributeValue("description"),
		OU:          ouFromDN(e.DN),
		MemberCount: len(e.GetAttributeValues("member")),
	}
}

// ouFromDN extracts the OU path from a distinguished name.
func ouFromDN(dn string) string {
	parts := strings.Split(dn, ",")
	var ous []string
	for _, p := range parts {
		if strings.HasPrefix(strings.ToUpper(p), "OU=") {
			ous = append(ous, strings.TrimPrefix(strings.TrimPrefix(p, "OU="), "ou="))
		}
	}
	return strings.Join(ous, "/")
}

// Windows FILETIME is 100-nanosecond intervals since 1601-01-01.
var winEpoch = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)

func parseWinFileTime(s string) *time.Time {
	if s == "" || s == "0" || s == "9223372036854775807" {
		return nil
	}
	ft, err := strconv.ParseInt(s, 10, 64)
	if err != nil || ft == 0 {
		return nil
	}
	t := winEpoch.Add(time.Duration(ft * 100))
	return &t
}

func toWindowsFileTime(t time.Time) int64 {
	return t.Sub(winEpoch).Nanoseconds() / 100
}
