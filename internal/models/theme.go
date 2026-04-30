package models

type BrandTheme struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	PrimaryColor      string  `json:"primary_color"`
	SecondaryColor    string  `json:"secondary_color"`
	AccentColor       string  `json:"accent_color"`
	FontName          string  `json:"font_name"`
	LogoURL           *string `json:"logo_url"`
	MascotDescription *string `json:"mascot_description"`
	ImageStyle        *string `json:"image_style"`
	Active            bool    `json:"active"`
}
