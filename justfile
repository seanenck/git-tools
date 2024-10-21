goflags := "-trimpath -buildmode=pie -mod=readonly -modcacherw -buildvcs=false"
objects := "target"

default: (build "git-uncommitted") (build "git-current-state") (build "git-dotfiles")

build target:
  @echo 'Building {{target}}...'
  mkdir -p {{objects}}
  go build {{goflags}} -o "{{objects}}/{{target}}" "cmd/{{target}}/"*.go

clean:
  rm -f "{{objects}}/"*
