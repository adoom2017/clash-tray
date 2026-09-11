.PHONY: build run test check clean
build:
	cargo build --release --locked
run:
	cargo run --locked
test:
	cargo test --locked
check:
	cargo fmt --check
	cargo clippy --all-targets --locked -- -D warnings
clean:
	cargo clean
