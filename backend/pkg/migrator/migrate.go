package migrator

type Entity interface {
	ID() int64
	Compareto(dst Entity) bool
}
