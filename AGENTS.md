* When adding support for a new api, make sure to:
  - Add an entry to the action enum
  - Add the entry at the end of the list, so that you don't renumber existing apis. Don't try to group apis together, just append them to end of the enum as you add them.
  - Make sure to update the init function so that the enum maps are updated accordingly.
* Before committing  make sure the follow all pass:
  - make fmt
  - make build
  - make test
  - make lint
