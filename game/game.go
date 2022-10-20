package game

// TODO - you should split this up by domain in the future
import (
	"regexp"

	"github.com/rs/zerolog/log"
)


const NumBodyTypes = 5
type Body struct {
	Type uint32
}

type Speech struct {
	Text string
	handledSent, handledRender bool
}

// handles the speech, returns true if the speech wasn't already handled
func (s *Speech) HandleSent() bool {
	if s.handledSent {
		return false
	}

	s.handledSent = true
	return true
}

func (s *Speech) HandleRender() bool {
	if s.handledRender {
		return false
	}

	s.handledRender = true
	return true
}


// This should probably be somewhere else
func FilterChat(msg string) string {
	match, err := regexp.MatchString(`^[\w!@#$%^&*()[{\]}'";:<>,.\/\?~\-_,.+=\\ ]+$`, msg)
	if err != nil {
		log.Error().Err(err).Msg("Regex Matching error")
		return "[This message was delete by moderator.]"
	}
	if match {
		return msg
	} else {
		return "[This message was delete by moderator.]"
	}
}
