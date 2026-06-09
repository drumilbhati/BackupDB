package db

import (
	"bufio"
	"io"
	"strings"
)

// SQLFilterReader is a reader that filters a SQL dump stream to only include specific tables.
type SQLFilterReader struct {
	scanner      *bufio.Scanner
	allowed      map[string]bool
	currentMatch bool
	isSQLite     bool
	buffer       []byte
	err          error
}

func NewSQLFilterReader(source io.Reader, tables []string, isSQLite bool) *SQLFilterReader {
	allowed := make(map[string]bool)
	for _, t := range tables {
		allowed[strings.ToLower(t)] = true
	}

	return &SQLFilterReader{
		scanner:      bufio.NewScanner(source),
		allowed:      allowed,
		currentMatch: len(tables) == 0, // If no tables specified, allow everything
		isSQLite:     isSQLite,
	}
}

func (r *SQLFilterReader) Read(p []byte) (n int, err error) {
	if len(r.buffer) > 0 {
		n = copy(p, r.buffer)
		r.buffer = r.buffer[n:]
		return n, nil
	}

	if r.err != nil {
		return 0, r.err
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		
		// Check for table boundaries
		tableName := extractTableName(line, r.isSQLite)
		if tableName != "" {
			if len(r.allowed) > 0 {
				r.currentMatch = r.allowed[strings.ToLower(tableName)]
			}
		}

		if r.currentMatch {
			lineBytes := []byte(line + "\n")
			n = copy(p, lineBytes)
			if n < len(lineBytes) {
				r.buffer = lineBytes[n:]
			}
			return n, nil
		}
	}

	r.err = r.scanner.Err()
	if r.err == nil {
		r.err = io.EOF
	}
	return 0, r.err
}

func extractTableName(line string, isSQLite bool) string {
	upper := strings.ToUpper(line)
	var patterns []string
	
	if isSQLite {
		patterns = []string{"CREATE TABLE ", "INSERT INTO ", "DROP TABLE "}
	} else {
		// MySQL uses backticks
		patterns = []string{"CREATE TABLE `", "INSERT INTO `", "DROP TABLE IF EXISTS `"}
	}

	for _, p := range patterns {
		if idx := strings.Index(upper, p); idx != -1 {
			start := idx + len(p)
			end := -1
			if isSQLite {
				// SQLite table names end with space or (
				endSpace := strings.Index(line[start:], " ")
				endParen := strings.Index(line[start:], "(")
				if endSpace != -1 && (endParen == -1 || endSpace < endParen) {
					end = endSpace
				} else {
					end = endParen
				}
			} else {
				// MySQL table names end with backtick
				end = strings.Index(line[start:], "`")
			}
			
			if end != -1 {
				return line[start : start+end]
			}
		}
	}
	return ""
}
