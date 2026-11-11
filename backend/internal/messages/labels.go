package messages

import (
	"database/sql"
	"fmt"
)

// selfLabel marks the viewer's own messages.
const selfLabel = "나"

// inquirerLetters turns an inquiry number into A, B, …, Z, AA, AB, … (docs/erd.md).
func inquirerLetters(n int64) string {
	var out []byte
	for n > 0 {
		n--
		out = append([]byte{byte('A' + n%26)}, out...)
		n /= 26
	}
	return string(out)
}

// counterpartyLabel is how the viewer sees the other party of a conversation.
// Buyers always see the seller; the seller sees the buyer's application alias
// once it exists and the inquiry alias before that.
func counterpartyLabel(viewerIsSeller bool, inquirer, applicant sql.NullInt64) string {
	if !viewerIsSeller {
		return "판매자"
	}
	if applicant.Valid {
		return fmt.Sprintf("익명 %d", applicant.Int64)
	}
	return "문의자 " + inquirerLetters(inquirer.Int64)
}
