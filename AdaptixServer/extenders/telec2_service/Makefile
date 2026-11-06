all: clean
	@ echo "    * Building TeleC2 service plugin"
	@ go test ./...
	@ mkdir dist
	@ cp config.yaml telec2.axs telec2-config.example.json README.md ./dist/
	@ GOEXPERIMENT=jsonv2,greenteagc go build -buildmode=plugin -ldflags="-s -w" -o ./dist/telec2_service.so pl_main.go
	@ echo "      done..."

clean:
	@ rm -rf dist
