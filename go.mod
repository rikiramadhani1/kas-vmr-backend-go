module github.com/vmr/kas-vmr-backend

go 1.22

replace golang.org/x/sys => github.com/golang/sys v0.19.0

replace golang.org/x/net => github.com/golang/net v0.24.0

replace golang.org/x/crypto => github.com/golang/crypto v0.22.0

replace golang.org/x/text => github.com/golang/text v0.14.0

require (
	github.com/SherClockHolmes/webpush-go v1.4.0
	github.com/emersion/go-imap v1.2.1
	github.com/emersion/go-imap-idle v0.0.0-20210907174914-db2568431445
	github.com/emersion/go-message v0.18.2
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/joho/godotenv v1.5.1
	github.com/labstack/echo/v4 v4.12.0
	github.com/redis/go-redis/v9 v9.6.1
	github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd
	golang.org/x/crypto v0.31.0
	gorm.io/driver/postgres v1.5.11
	gorm.io/gorm v1.25.12
)

require (
	github.com/emersion/go-sasl v0.0.0-20200509203442-7bfe0ed36a21 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	golang.org/x/time v0.5.0 // indirect
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-playground/validator/v10 v10.22.1
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v5 v5.5.5 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace gorm.io/gorm => github.com/go-gorm/gorm v1.25.12

replace gorm.io/driver/postgres => github.com/go-gorm/postgres v1.5.11

replace gopkg.in/yaml.v3 => github.com/go-yaml/yaml/v3 v3.0.1

replace gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20161208181325-20d25e280405

replace golang.org/x/sync => github.com/golang/sync v0.1.0

replace golang.org/x/time => github.com/golang/time v0.5.0

replace golang.org/x/tools => github.com/golang/tools v0.6.0

replace golang.org/x/mod => github.com/golang/mod v0.8.0

replace golang.org/x/term => github.com/golang/term v0.19.0
