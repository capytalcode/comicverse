PORT?=8080

lint:
	golangci-lint run .

fmt:
	go fmt .
	golangci-lint run --fix .

dev/server:
	go run github.com/joho/godotenv/cmd/godotenv@v1.5.1 \
		go run github.com/air-verse/air@v1.52.2 \
			--build.cmd "go build -o tmp/bin/main ./cmd" \
			--build.bin "tmp/bin/main" \
			--build.exclude_dir "node_modules" \
			--build.include_ext "go" \
			--build.stop_on_error "false" \
			--misc.clean_on_exit true \
			-- -dev -port $(PORT) -hostname 0.0.0.0

dev/assets:
	tailwindcss \
		-i ./assets/stylesheets/tailwind.css \
		-o ./assets/stylesheets/out.css \
		--watch

dev:
	$(MAKE) -j2 dev/assets dev/server

dev/debug:
	$(MAKE) -j2 debug dev/assets

debug:
	dlv debug -l 127.0.0.1:38697 \
		--continue \
		--accept-multiclient \
		--headless \
		./cmd -- -dev -port $(PORT) -hostname 0.0.0.0

build/assets:
	tailwindcss \
		-i ./assets/stylesheets/tailwind.css \
		-o ./assets/stylesheets/out.css \
		--minify

build: build/assets
	go build -o ./.dist/app .

run: build
	./.dist/app

epub/example:
	cd ./.epub/example; zip ./example.epub ./META-INF/container.xml ./OEBPS/* ./OEBPS/**/* ./mimetype

epub/example/server:
	cd ./.epub/example; http-server

calibre:
	mkdir -p ./tmp/calibre-library
	calibre \
		--no-update-check \
		--with-library=./tmp/calibre-library \
		./.epub/example/example.epub

clean:
	# Remove generated directories
	if [[ -d ".dist" ]]; then rm -r ./.dist; fi
	if [[ -d "tmp" ]]; then rm -r ./tmp; fi
	if [[ -d "bin" ]]; then rm -r ./bin; fi
