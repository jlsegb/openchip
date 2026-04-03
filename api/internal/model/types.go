package model

import "time"

type Owner struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name"`
	Phone     *string    `json:"phone,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Pet struct {
	ID             string     `json:"id"`
	OwnerID        string     `json:"owner_id"`
	ChipPK         string     `json:"chip_pk"`
	ChipIDRaw      string     `json:"chip_id_raw"`
	ChipNormalized string     `json:"chip_id_normalized"`
	Manufacturer   string     `json:"manufacturer_hint"`
	PetName        string     `json:"pet_name"`
	Species        string     `json:"species"`
	Breed          *string    `json:"breed,omitempty"`
	Color          *string    `json:"color,omitempty"`
	DateOfBirth    *time.Time `json:"date_of_birth,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	PhotoURL       *string    `json:"photo_url,omitempty"`
	Active         bool       `json:"active"`
	RegisteredAt   time.Time  `json:"registered_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type LookupRegistration struct {
	OwnerID      string  `json:"owner_id"`
	PetName      string  `json:"pet_name"`
	Species      string  `json:"species"`
	Breed        *string `json:"breed,omitempty"`
	Color        *string `json:"color,omitempty"`
	OwnerName    string  `json:"owner_name"`
	OwnerFirst   string  `json:"owner_first_name"`
	OwnerPhone   *string `json:"owner_phone,omitempty"`
	OwnerEmail   *string `json:"owner_email,omitempty"`
	Manufacturer string  `json:"manufacturer_hint"`
}
