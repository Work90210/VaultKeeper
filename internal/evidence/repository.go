package evidence

type Repository interface {
	EvidenceReader
	EvidenceWriter
}
