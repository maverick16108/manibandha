package web

import (
	"strings"
	"time"

	"manibandha/internal/models"
)

// jsonDate декодирует "YYYY-MM-DD" (или null/"") в *time.Time.
type jsonDate struct {
	t *time.Time
}

func (d *jsonDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		d.t = nil
		return nil
	}
	tt, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	d.t = &tt
	return nil
}

// discipleInput — тело create/update. nil-указатель = поле не передано (exclude_unset).
type discipleInput struct {
	SpiritualName      *string   `json:"spiritual_name"`
	MaterialName       *string   `json:"material_name"`
	PhotoURL           *string   `json:"photo_url"`
	Phone              *string   `json:"phone"`
	Email              *string   `json:"email"`
	Messenger          *string   `json:"messenger"`
	Country            *string   `json:"country"`
	Region             *string   `json:"region"`
	City               *string   `json:"city"`
	TempleID           *int      `json:"temple_id"`
	Gender             *string   `json:"gender"`
	MaritalStatus      *string   `json:"marital_status"`
	DateOfBirth        *jsonDate `json:"date_of_birth"`
	InitiationStatus   *string   `json:"initiation_status"`
	PranamaDate        *jsonDate `json:"pranama_date"`
	HarinamaDate       *jsonDate `json:"harinama_date"`
	HarinamaName       *string   `json:"harinama_name"`
	BrahmanDate        *jsonDate `json:"brahman_date"`
	Seva               *string   `json:"seva"`
	CurrentActivity    *string   `json:"current_activity"`
	MentorID           *int      `json:"mentor_id"`
	MentorName         *string   `json:"mentor_name"`
	IsMentor           *bool     `json:"is_mentor"`
	RecommendedBy      *string   `json:"recommended_by"`
	ApplicationDate    *jsonDate `json:"application_date"`
	ReadyForPranama    *bool     `json:"ready_for_pranama"`
	ReadyForInitiation *bool     `json:"ready_for_initiation"`
	Notes              *string   `json:"notes"`
}

// applyTo заполняет модель при создании.
func (in *discipleInput) applyTo(d *models.Disciple) {
	if in.SpiritualName != nil {
		d.SpiritualName = in.SpiritualName
	}
	if in.MaterialName != nil {
		d.MaterialName = *in.MaterialName
	}
	if in.PhotoURL != nil {
		d.PhotoURL = in.PhotoURL
	}
	if in.Phone != nil {
		d.Phone = in.Phone
	}
	if in.Email != nil {
		d.Email = in.Email
	}
	if in.Messenger != nil {
		d.Messenger = in.Messenger
	}
	if in.Country != nil {
		d.Country = in.Country
	}
	if in.Region != nil {
		d.Region = in.Region
	}
	if in.City != nil {
		d.City = in.City
	}
	if in.TempleID != nil {
		d.TempleID = in.TempleID
	}
	if in.Gender != nil {
		d.Gender = in.Gender
	}
	if in.MaritalStatus != nil {
		d.MaritalStatus = in.MaritalStatus
	}
	if in.DateOfBirth != nil {
		d.DateOfBirth = in.DateOfBirth.t
	}
	if in.InitiationStatus != nil && *in.InitiationStatus != "" {
		d.InitiationStatus = *in.InitiationStatus
	}
	if in.PranamaDate != nil {
		d.PranamaDate = in.PranamaDate.t
	}
	if in.HarinamaDate != nil {
		d.HarinamaDate = in.HarinamaDate.t
	}
	if in.HarinamaName != nil {
		d.HarinamaName = in.HarinamaName
	}
	if in.BrahmanDate != nil {
		d.BrahmanDate = in.BrahmanDate.t
	}
	if in.Seva != nil {
		d.Seva = in.Seva
	}
	if in.CurrentActivity != nil {
		d.CurrentActivity = in.CurrentActivity
	}
	if in.MentorID != nil {
		d.MentorID = in.MentorID
	}
	if in.MentorName != nil {
		d.MentorName = in.MentorName
	}
	if in.IsMentor != nil {
		d.IsMentor = *in.IsMentor
	}
	if in.RecommendedBy != nil {
		d.RecommendedBy = in.RecommendedBy
	}
	if in.ApplicationDate != nil {
		d.ApplicationDate = in.ApplicationDate.t
	}
	if in.ReadyForPranama != nil {
		d.ReadyForPranama = *in.ReadyForPranama
	}
	if in.ReadyForInitiation != nil {
		d.ReadyForInitiation = *in.ReadyForInitiation
	}
	if in.Notes != nil {
		d.Notes = in.Notes
	}
}

// updateMap собирает переданные поля для PATCH.
func (in *discipleInput) updateMap() map[string]any {
	m := map[string]any{}
	putStr := func(col string, p *string) {
		if p != nil {
			m[col] = *p
		}
	}
	putStr("spiritual_name", in.SpiritualName)
	putStr("material_name", in.MaterialName)
	putStr("photo_url", in.PhotoURL)
	putStr("phone", in.Phone)
	putStr("email", in.Email)
	putStr("messenger", in.Messenger)
	putStr("country", in.Country)
	putStr("region", in.Region)
	putStr("city", in.City)
	putStr("gender", in.Gender)
	putStr("marital_status", in.MaritalStatus)
	putStr("harinama_name", in.HarinamaName)
	putStr("seva", in.Seva)
	putStr("current_activity", in.CurrentActivity)
	putStr("mentor_name", in.MentorName)
	putStr("recommended_by", in.RecommendedBy)
	putStr("notes", in.Notes)
	if in.InitiationStatus != nil {
		m["initiation_status"] = *in.InitiationStatus
	}
	if in.TempleID != nil {
		m["temple_id"] = *in.TempleID
	}
	if in.MentorID != nil {
		m["mentor_id"] = *in.MentorID
	}
	if in.IsMentor != nil {
		m["is_mentor"] = *in.IsMentor
	}
	if in.ReadyForPranama != nil {
		m["ready_for_pranama"] = *in.ReadyForPranama
	}
	if in.ReadyForInitiation != nil {
		m["ready_for_initiation"] = *in.ReadyForInitiation
	}
	if in.DateOfBirth != nil {
		m["date_of_birth"] = in.DateOfBirth.t
	}
	if in.PranamaDate != nil {
		m["pranama_date"] = in.PranamaDate.t
	}
	if in.HarinamaDate != nil {
		m["harinama_date"] = in.HarinamaDate.t
	}
	if in.BrahmanDate != nil {
		m["brahman_date"] = in.BrahmanDate.t
	}
	if in.ApplicationDate != nil {
		m["application_date"] = in.ApplicationDate.t
	}
	return m
}
