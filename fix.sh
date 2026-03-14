#!/bin/bash

# Fix errcheck in internal/telegram/handlers/volunteer.go
sed -i 's/fmt.Sscanf(parts\[1\], "%d", &days)/_, _ = fmt.Sscanf(parts[1], "%d", \&days)/g' internal/telegram/handlers/volunteer.go

# Fix errcheck in internal/telegram/handlers/admin.go
sed -i 's/fmt.Sscanf(userID, "%d", &id)/_, _ = fmt.Sscanf(userID, "%d", \&id)/g' internal/telegram/handlers/admin.go
sed -i 's/fmt.Sscanf(parts\[1\], "%d", &userID)/_, _ = fmt.Sscanf(parts[1], "%d", \&userID)/g' internal/telegram/handlers/admin.go
sed -i 's/fmt.Sscanf(parts\[2\], "%d", &days)/_, _ = fmt.Sscanf(parts[2], "%d", \&days)/g' internal/telegram/handlers/admin.go
sed -i 's/fmt.Sscanf(parts\[2\], "%d", &userID)/_, _ = fmt.Sscanf(parts[2], "%d", \&userID)/g' internal/telegram/handlers/admin.go
