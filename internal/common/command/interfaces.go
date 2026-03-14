package command

import "context"

// Runner интерфейс для выполнения команд
type Runner interface {
	// Run выполняет команду с автоматической подстановкой commandPrefix и включением verbose режима
	// Возвращает stdout, stderr, error)
	Run(ctx context.Context, args []string, opts ...Option) (string, string, error)
}
