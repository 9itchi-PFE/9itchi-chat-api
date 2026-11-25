package core

// TeamID is the seat of the support team in every conversation. Support is a team inbox: a student talks to the
// team, any admin reads and answers from this seat, and the admin who writes stays the sender of the message.
const TeamID = "00000000-0000-4000-8000-000000000000"

// SeatOf is the participant a caller acts as: the team for an admin, themselves for a student.
func SeatOf(userID, role string) string {
	if role == "admin" {
		return TeamID
	}
	return userID
}
