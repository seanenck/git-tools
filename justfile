goflags := "-trimpath -buildmode=pie -mod=readonly -modcacherw -buildvcs=false"

default: (build "git-uncommitted") (build "git-current-state")

build target:
  @echo 'Building {{target}}...'
  cp src/* "cmd/{{target}}/"
  go build {{goflags}} -o {{target}} "cmd/{{target}}/"*

clean:
  rm -f "git-"*

#clean:
#  rm -f {{objects}}
#
#$(OBJECTS): cmd/**/main.go src/*.go go.*
#	cp src/* cmd/$@/
#	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ cmd/$@/*
#
#clean:
#	rm -f $(OBJECTS)