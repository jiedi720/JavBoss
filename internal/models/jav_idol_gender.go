package models

// Actor genders share the jav_idol table. Every actor created by the legacy
// scrape/enrichment pipeline is female; male actors are managed manually.
const (
	ActorGenderFemale = "female"
	ActorGenderMale   = "male"
)

// IsValidActorGender reports whether gender is one of the supported values.
func IsValidActorGender(gender string) bool {
	return gender == ActorGenderFemale || gender == ActorGenderMale
}

// NormalizeActorGender treats empty (legacy) values as female so that existing
// rows and callers keep their original behaviour.
func NormalizeActorGender(gender string) string {
	if gender == "" {
		return ActorGenderFemale
	}
	return gender
}
