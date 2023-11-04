package models

type Rating struct {
	Likes    int
	Dislikes int
}

type UsernameRating struct {
	Username string
	*Rating
}
