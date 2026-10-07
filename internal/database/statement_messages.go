package database

import (
	"context"
	"encoding/json"
	"strconv"
)

func statementMessageSourceIn(ctx context.Context, q identityReader, id string) (MessageSource, error) {
	src := MessageSource{Kind: "STATEMENT", ID: id}
	var target, file, digest string
	var publicationVersion, fileVersion int
	err := q.QueryRowContext(ctx, `SELECT f.id,f.title,f.sha256,f.version,sp.version,sp.target_json
	 FROM statement_publications sp JOIN statement_groups g ON g.current_publication_id=sp.id
	 JOIN statement_files f ON f.id=sp.file_id
	 WHERE sp.id=? AND sp.state='PUBLISHED' AND f.state='APPROVED' AND f.validation='AVAILABLE'`, id).
		Scan(&file, &src.Title, &digest, &fileVersion, &publicationVersion, &target)
	if err != nil {
		return src, err
	}
	var audience MessageTarget
	if err = json.Unmarshal([]byte(target), &audience); err != nil {
		return src, err
	}
	src.PublicationTarget, src.Audience, src.Wing = &audience, audience.Kind, audience.Wing
	src.Version = strconv.Itoa(publicationVersion) + "." + strconv.Itoa(fileVersion) + "." + digest
	src.Link = "/#statements?statement=" + file
	return src, nil
}

// A home picker should not offer another relationship outside a frozen
// publication when the same person owns or occupies several homes.
func statementMessageHomeChoice(person messagePerson, home messageMembership, src MessageSource) bool {
	if src.Kind != "STATEMENT" {
		return true
	}
	if src.PublicationTarget == nil {
		return false
	}
	target := *src.PublicationTarget
	if target.Kind == "PEOPLE" {
		return messageSourceMatches(person, src)
	}
	return messageTargetMatches(messagePerson{ID: person.ID, Homes: []messageMembership{home}}, target)
}
