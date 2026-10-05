# Configurator 

An easy way of managing variables & secrets for Terraform. Using hexagonal architecture, this lets you have multiple variable storage providers (e.g. AWS SSM Parameters) & multiple output formats (e.g. Terraform Variables)


## Initialize 

```
$ configurator init 
```

This will create a `.configurator` file:

```yaml
PROVIDER=aws
PROVIDER_PROFILE=aws_profile
PROJECT_NAME=name
VARIABLE_BASE_PATH=/configurator/name
```

## Usage

```
$ configurator set "key" --sensitive (bool flag)
$ configurator get "key"
$ configurator list (lists all keys + variables.. use astricks if sensitive for value)
$ configurator load <.json file>
```

Load will be used to inject an entire `.json` file into the providers storage.

E.g., if SSM, it will be at:

```
$VARIABLE_BASE_PATH/<key> == VALUE
```

Dynamically marked as `SecureString` if it is sensitive.

## Terraform

```
$ configurator sync
```

This will generate a `configurator.tf` file where each variable is referenced, and then a local version made:

```terraform
data "aws_ssm_parameter" "configurator_$KEY" {
  name            = "$VARIABLE_BASE_PATH/<key>"
  with_decryption = true // IF sensitive 
}

locals {
  # Wrap the value with the sensitive() function if needed.
  configurator_$KEY = data.aws_ssm_parameter.configurator_$KEY.value
}
```

Importantly, the locals should be used throughout TF so that the SSM provider stays dynamic.
