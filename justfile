test:
    go test ./... -cover -coverprofile coverage
build-compiler:
    go build -o sydney
[working-directory: 'sydney_rt']
build-rt:
    cargo build --release
build-smoke:
    llc -filetype=obj smoke.ll -o smoke.o
    clang smoke.o -Lsydney_rt/target/release -lsydney_rt -o smoke

clean-smoke:
    rm smoke.o
build:
    just build-rt
    just build-smoke
    just clean-smoke
