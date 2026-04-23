package user

import "time"

// UserResponse is the public-safe shape returned to callers.
// Never expose PasswordHash, DeletedAt, or internal flags.
type UserResponse struct {
	ID              string           `json:"id"`
	Email           string           `json:"email"`
	Name            string           `json:"name"`
	Role            string           `json:"role"`
	IsEmailVerified bool             `json:"isEmailVerified"`
	LastLoginAt     *time.Time       `json:"lastLoginat,omitempty"`
	Profile         *ProfileResponse `json:"profile,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
}

type ProfileResponse struct {
	Bio       string `json:"bio,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	Location  string `json:"location,omitempty"`
	Website   string `json:"website,omitempty"`
}

type UpdateProfileRequest struct {
	Name      string `json:"name"       validate:"omitempty,min=2,max=100"`
	Bio       string `json:"bio"        validate:"omitempty,max=500"`
	AvatarURL string `json:"avatarUrl" validate:"omitempty,url,max=500"`
	Location  string `json:"location"   validate:"omitempty,max=100"`
	Website   string `json:"website"    validate:"omitempty,url,max=200"`
}

type ListUsersRequest struct {
	Page   int    `query:"page"     validate:"omitempty,min=1"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Search string `query:"search"   validate:"omitempty,max=100"`
	Role   string `query:"role"     validate:"omitempty,oneof=user admin"`
}

func (r *ListUsersRequest) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.Limit < 1 || r.Limit > 100 {
		r.Limit = 20
	}
}

// toResponse converts a User model to the safe public response DTO.
func toResponse(u *User) UserResponse {
	resp := UserResponse{
		ID:              u.ID.String(),
		Email:           u.Email,
		Name:            u.Name,
		Role:            u.Role,
		IsEmailVerified: u.IsEmailVerified,
		LastLoginAt:     u.LastLoginAt,
		CreatedAt:       u.CreatedAt,
	}
	if u.Profile != nil {
		resp.Profile = &ProfileResponse{
			Bio:       u.Profile.Bio,
			AvatarURL: u.Profile.AvatarURL,
			Location:  u.Profile.Location,
			Website:   u.Profile.Website,
		}
	}
	return resp
}
