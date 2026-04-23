package ollama

type Config struct {
	URL       string
	Model     string
	Dims      int
	BatchSize int
}
