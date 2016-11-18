<p align="center">
  <img src="https://avatars3.githubusercontent.com/u/12045289?v=3&u=27e30e812e6806e20a36c16a0c5b43b3796c7c03&s=400" alt="Hungrilla logo" />
  <h1 align="center">Hungrilla</h1>
</p>

### Setup

```bash
# check version of go, should be greater than 1.6 and vendoring enabled
λ go version
go version go1.7 linux/amd64

# glide should be installed
λ glide -v
glide version v0.11.1

# get the project source
λ go get github.com/umayr/hungrilla

# change directory to source
λ cd $GOPATH/src/github.com/umayr/hungrilla

# install dependencies
λ glide install

# create development configurations
λ cp conf/development.yaml.sample conf/development.yaml

# execute the crawler
λ go run cmd/crawler/main.go
```
