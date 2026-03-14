sed -i 's|"math/rand"|"math/rand/v2"|' internal/notification/periodic_chore_reminder.go
gofmt -w internal/notification/periodic_chore_reminder.go
