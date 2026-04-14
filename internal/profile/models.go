package profile

import "time"

type Profile struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Bio         string    `json:"bio"`
	Timezone    string    `json:"timezone"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UpdateProfileInput struct {
	DisplayName *string `json:"display_name,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
}
