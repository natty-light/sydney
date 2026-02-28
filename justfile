test:
    go test ./... -cover -coverprofile coverage
build-compiler:
    go build -o sydney
[working-directory: 'sydney_rt']
build-rt:
    cargo build --release
build-smoke:
    ./sydney compile smoke.sy
    llc -filetype=obj smoke.ll -o smoke.o
    clang smoke.o -Lsydney_rt/target/release -lsydney_rt -o smoke

build-smoke-opt:
    ./sydney compile smoke.sy
    opt -O2 -S smoke.ll -o smoke_opt.ll
    llc -filetype=obj smoke_opt.ll -o smoke.o
    clang smoke.o -Lsydney_rt/target/release -lsydney_rt -o smoke

clean-smoke:
    rm smoke.o
build:
    just build-compiler
    just build-rt
compile:
    just build-smoke
    just clean-smoke
compile-opt:
    just build-smoke-opt
    just clean-smoke