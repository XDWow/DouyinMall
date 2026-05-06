package job

type Job interface {
	Run() error
	Name() string
}
