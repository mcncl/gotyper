package usermodel

import (
	"time"
)

type UserData struct {
	User *UserDataUser `json:"user,omitempty"`
}

type UserDataUser struct {
	Active      bool                     `json:"active"`
	CreatedAt   time.Time                `json:"created_at"`
	Email       string                   `json:"email"`
	Id          int64                    `json:"id"`
	Name        string                   `json:"name"`
	Preferences *UserDataUserPreferences `json:"preferences,omitempty"`
	Profile     *UserDataUserProfile     `json:"profile,omitempty"`
	Roles       *[]string                `json:"roles,omitempty"`
	Stats       *UserDataUserStats       `json:"stats,omitempty"`
}

type UserDataUserPreferences struct {
	Notifications *UserDataUserPreferencesNotifications `json:"notifications,omitempty"`
	Theme         string                                `json:"theme"`
	Timezone      string                                `json:"timezone"`
}

type UserDataUserPreferencesNotifications struct {
	Email bool `json:"email"`
	Push  bool `json:"push"`
}

type UserDataUserProfile struct {
	AvatarUrl string                     `json:"avatar_url"`
	Bio       string                     `json:"bio"`
	Location  string                     `json:"location"`
	Social    *UserDataUserProfileSocial `json:"social,omitempty"`
}

type UserDataUserProfileSocial struct {
	Github   string `json:"github"`
	Linkedin string `json:"linkedin"`
	Twitter  string `json:"twitter"`
}

type UserDataUserStats struct {
	Followers int64 `json:"followers"`
	Following int64 `json:"following"`
	Posts     int64 `json:"posts"`
}
