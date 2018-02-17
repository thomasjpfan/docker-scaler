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

1. Fork repo

```bash
git clone https://github.com/thomasjpfan/docker-scaler
```

2. Install [dep](https://github.com/golang/dep).

!!! tip
    When adding a feature that uses a vendored dependency, you may need to run `dep ensure` to pull in those files. The `Gopkg.toml` file sets `unused-packages = true` which prunes out files from directories that do not appear in the package import graph

3. Unit Testing

```bash
make unit_test
```

4. Build

```bash
make build
```

5. Test

```bash
make deploy_test
make integration_test
```

6. Cleanup

```bash
make undeploy_test
```
