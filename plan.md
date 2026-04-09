1. **Modify `assignChore` Signature**: Update `assignChore` in `internal/telegram/handlers/chore.go` to accept an additional string parameter `specificUserName`:
   ```go
   func (h *Handlers) assignChore(chatID int64, fromUserID int64, description string, specificUserName string) (tgbotapi.MessageConfig, error)
   ```
2. **Update `HandleChore` Logic**: Modify parsing in `HandleChore` to handle the specific participant suffix (e.g. `/Crusader`). We can check for a suffix by parsing `/name` at the end of the chore description. We should extract `specificUserName` and call `assignChore(..., specificUserName)`.
   Since an admin could write `/<N>d` and `/<name>` in any order, or just `/<name>`, we should parse them using regex.
   Specifically, look for `\s+/([a-zA-Z0-9_]+)$` recursively. If it matches `/[0-9]+d`, it's an interval. Otherwise, it is assumed to be a specific user.
3. **Refactor `assignChore`**: Inside `assignChore`, if `specificUserName` is not empty, perform a case-insensitive search through `users` (which contains active users from the database) to find a user matching `specificUserName`. If a user is found, skip the random weighted selection and assign the chore directly to them. If the user is not found, return a `MessageConfig` indicating that the specific user was not found.
4. **Update existing `assignChore` calls**: Update the 3 existing calls to `assignChore` in `chore.go` to pass `specificUserName`. For interactive mode, we can parse it as well, or just pass `""` if we want to restrict this feature to inline arguments. Let's parse it in `HandleChoreInteractive` too so it's consistent.
5. **Pre-commit**: Follow `pre_commit_instructions` before submitting to ensure `go test` and `golangci-lint` pass.
