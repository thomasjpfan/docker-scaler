# Feedback and Contribution

The *Docker Scaler* project welcomes, and depends, on contributions from developers and users in the open source community. Contributions can be made in a number of ways, a few examples are:

* Code patches or new features via pull requests
* Documentation improvements
* Bug reports and patch reviews

## Reporting an Issue

Feel fee to [create a new issue](https://github.com/thomasjpfan/docker-scaler/issues). Include as much detail as you can.

If an issue is a bug, please provide steps to reproduce it.

If an issue is a request for a new feature, please specify the use-case behind it.

## Contributing To The Project

This project is developed using **Test Driven Development**. When a new feature is added please run through the testing procedure:

### Fork repo

```bash
git clone https://github.com/thomasjpfan/docker-scaler
```

### Unit Testing

```bash
go test ./... --run UnitTest -v
```

### Build

```bash
docker image build -t thomasjpfan/docker-scaler:beta .
```

### Test

``` bash
export TRAVIS_COMMIT=beta
docker stack deploy -c stacks/docker-scaler-test.yml test

./scripts/integration_test.sh
```
