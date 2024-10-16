OBJECTS := git-uncommitted git-current-state
GOFLAGS := -trimpath -buildmode=pie -mod=readonly -modcacherw -buildvcs=false

all: $(OBJECTS)

$(OBJECTS): cmd/**/main.go src/*.go go.*
	cp src/* cmd/$@/
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ cmd/$@/*

clean:
	rm -f $(OBJECTS)
