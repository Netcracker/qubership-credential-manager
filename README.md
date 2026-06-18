# qubership-credential-manager

This component is used to support credentials change functionality.

Credential manager consists of several packages.

# Coniguration
#test
## environment variables
The next environment variables must be configured:

`IS_HOOK` - Required for hook module `IsHook() bool` function.  
`SECRET_NAMES` - List of coma separated secret names to work with.  
`HOOK_NAME` - Prefix for hook Job objects. By default `credentials-saver`.  

# Modules

## hook
This module is used in pre-deploy hook for creation of secret old version.

API:

`PrepareOldCreds(secrets []string)` - The function accepts slice of secret names as an argument.
New secrets with the same content and name with postfix `-old` will be created for all of the provided secrets.
Secrets also will be locked with `locked-for-watcher=true` annotation on them.

`ClearHooks()` - This function deletes all Kubernetes Job and Pod objects in current namespace with prefix from `HOOK_NAME` environment variable.

## informer
This module allows you to create watcher for secret.

API:

`Watch(secretNames []string, reconcileFunc func())` - The function accepts slice of secret names for watching and function which triggers reconcile.
After method execution whatchers will be created for selected secrets. One watcher per secret. On each secret change `reconcileFunc` function will be triggered. (Except the case when secret is "Locked"). If watcher is already present for a secret, new watcher won't be created.

## manager
This module provides functionality to define secret change, and perform credentials update. Functions for setting secret hash also included.

API:

`AreCredsChanged(secretNames []string) (bool, error)` - This function accepts slice of secret names. If at least one of the secrets was changed,
this function returns `true`.

`ActualizeCreds(secretName string, changeCredsFunc func(newSecret, oldSecret *corev1.Secret) error) error` - The function accepts secret name and the function for credentials change. If secret data has diff `changeCredsFunc` function will be executed. After `changeCredsFunc` function execution secret with postfix `-old` will be updated with new data from secret with `secretName` name. At the end `secretName` secret will be unlocked by setting `locked-for-watcher=false` annotation.

`GetAnnotationName(id int) string` - This function provides annotation name for secret hash based on `id`.

`CalculateSecretDataHash(secretName string) (string, error)` - This function provides sha256 hashsum for `secretName` secret data.

`AddAnnotationsToPodTemplate(template *corev1.PodTemplateSpec, annotations map[string]string)` - This function merge `annotations` with Pod Template Spec existing annotations.

`AddCredHashToPodTemplate(secretNames []string, template *corev1.PodTemplateSpec) error` - This function calculates secret hashes and sets them in  Pod Template Spec annotations.

`SetOwnerRefForSecretCopies(secretNames []string, ownerRef []metav1.OwnerReference) error` - The function sets provided owner reference for secret copies with `-old` prefix, created by operator or pre-deploy hook.
