package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidFormat = errors.New("message must contain steam URL and target price separated by space")
	ErrInvalidURL    = errors.New("invalid steam store application URL")
	ErrInvalidPrice  = errors.New("target price must be a positive integer")
)

var steamAppRegex = regexp.MustCompile(`store\.steampowered\.com/app/(\d+)`)

type UserRequest struct {
	AppID       int
	TargetPrice int
}

// ParseUserMessage parses a string in the format "<Steam link> <price>"
func ParseUserMessage(text string) (UserRequest, error) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) != 2 {
		return UserRequest{}, ErrInvalidFormat
	}

	rawURL, rawPrice := fields[0], fields[1]

	matches := steamAppRegex.FindStringSubmatch(rawURL)
	if len(matches) < 2 {
		return UserRequest{}, ErrInvalidURL
	}

	appID, err := strconv.Atoi(matches[1])
	if err != nil || appID <= 0 {
		return UserRequest{}, ErrInvalidURL
	}

	price, err := strconv.Atoi(rawPrice)
	if err != nil || price <= 0 {
		return UserRequest{}, ErrInvalidPrice
	}

	return UserRequest{
		AppID:       appID,
		TargetPrice: price,
	}, nil
}