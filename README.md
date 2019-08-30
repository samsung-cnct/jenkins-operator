# Jenkins operator
[![Build Status](https://jenkins.cnct.io/buildStatus/icon?job=jenkins-operator/master)](https://jenkins.cnct.io/job/jenkins-operator/job/master/)

## Project status: alpha

Major planned features have been completed, and while no breaking API changes are currently planned, we reserve the right to address bugs and API changes in a backwards incompatible way before the project is declared stable.

We expect to consider the jenkins operator stable soon; backwards incompatible changes will not be made once the project reaches stability.

## Overview

The jenkins operator manages Jenkins instances deployed to [Kubernetes][k8s-home] and automates tasks related to operating a Jenkins server

- [Create and Destroy instances](#create-and-destroy-instances)
- [Configure jenkins via Jenkins-Configuration-As-code plugin](#managing-jenkins-resources)

## Requirements

- Kubernetes 1.9+
- Jenkins 2.1+

## Getting started

### Deploy jenkins operator

```bash
$ helm repo add cnct https://charts.cnct.io
$ helm repo update
$ helm install cnct/jenkins-operator --name jenkins-operator
```

or get it from the incubator, __which might not be as up-to-date as our own chart repo__:

```bash
helm repo add incubator https://kubernetes-charts-incubator.storage.googleapis.com/
$ helm repo update
$ helm install incubator/jenkins-operator --name jenkins-operator
```

or install from source:

```bash
$ helm install deployments/helm/jenkins-operator --name jenkins-operator
```

### Create a secret with Jenkins admin credentials
```bash
$ kubectl create -f config/samples/jenkinssecret.yaml 
```

A Kubernetes secret called `jenkins-test` with `JENKINS_ADMIN_USER` = `admin` and `JENKINS_ADMIN_PASSWORD` = `password` will be created

### Create a Jenkins instance
```bash
$ kubectl create -f config/samples/jenkins_v1alpha2_jenkinsinstance.yaml
```

This will create a Jenkins Kubernetes deployment along with a `NodePort` Kubernetes service, a 1Gb Kubernetes persistent volume for job storage, as well a few other Kubernetes objects.
This instance will have a few default plugins pre-installed, along with a `kubernetes` plugin at latest version.

You can visit this instance by checking the NodePort service port number, and then going to the ip of one of the cluster nodes at that port number.  
For example, if you are running in `minikube`:

```bash
$ minikube ip
192.168.64.19

$ $ kubectl get svc
  NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                          AGE
  jenkins      NodePort    10.98.209.217   <none>        8080:31363/TCP,50000:30997/TCP   7d
  kubernetes   ClusterIP   10.96.0.1       <none>        443/TCP                          7d
```

So, you should be able to point your browser to `http://192.168.64.19:31363` and login with `admin:password` credentials

## Configuration
## Create and Destroy instances

Instance creation and destruction is controlled through `JenkinsInstance` Kubernetes custom resources:

```
apiVersion: jenkins.jenkinsoperator.samsung-cnct.github.com/v1alpha2
kind: JenkinsInstance
metadata:
    labels:                                         # dictionary of Kubernetes labels
        stuff.cnct.io: "woo"
    name: jenkinsinstance-sample                    # Name for this Jenkins instance object
spec:
    image: "jenkins/jenkins:lts"                    # docker image with tag for this jenkins deployment
    imagepullpolicy: Always                         # image pull policy
    imagepullsecrets:                               # image pull secrets
        - name: myregistrykey
    resources:                                      # resources and limits config
        requests:
            memory: "64Mi"
            cpu: "250m"
        limits:
            memory: "128Mi"
            cpu: "500m"
    env:                                            # dictionary of environment variables to be set in the jenkins instance
        SOME_ENV: "test"
    plugins:                                        # dictionary of Jenkins plugins to install PRIOR to Jenkins startup
        - id:  config-file-provider                 # plugin id
          version: "3.1"                            # plugin version. Follows the format at https://github.com/jenkinsci/docker#plugin-version-format
    cascconfig:                                     # jenkins configuration as code plugin (JCASC) (https://github.com/jenkinsci/configuration-as-code-plugin) config
        configmap: jenkins-config                   # name of ConfigMap in the same namespace. Keys will be converted to JCASC config files. Takes precedence over configstring
        configstring: |                             # inline JCASC config. Will be converted to jenkins.yaml JCASC config file
            jenkins:
                ...
    cascsecret: jenkins-admin-secret                # name of a secret in the same namespace. Keys will be converted to files used for token replacement in JCASC config files
                                                    # per https://github.com/jenkinsci/configuration-as-code-plugin#handling-secrets
    groovysecret: jenkins-groovy-secret             # name of a secret in the same namespace. Keys will be mounted as .groovy files into ${JENKINS_HOME}/init.groovy.d and ran on startup
    annotations:                                    # Jenkins deployment annotations
        cnct.io/annotation: "test"
        cnct.io/other-annotation: "other test"
    executors: 2                                    # Number of Jenkins executors to setup
    adminsecret: jenkins-admin-secret               # name of the pre-existing kubernetes secret with admin user credentials
    service:                                        # Kubernetes service options. If not present, default service is created
        name: jenkins                               # service name
        servicetype: NodePort                       # service type
        nodeport: 30348                             # optional fixed nodeport
        annotations:
          cnct.io/service-annotation: "test"        # service annotations
    serviceaccount: jenkins                         # Kubernetes service account name for jenkins deployment
    storage:                                        # storage options
        jobspvc: jenkins                            # Name of PVC for job storage. If does not exist it will be created. If name is not specified, EmptyDir is used
        jobspvcspec:                                # If PVC is to be created, use this spec
            accessModes:                            # PVC access modes
                - ReadWriteOnce
            resources:                              # PVC requests
                requests:
                    storage: 1Gi
    affinity:                                       # k8s affinity configuration for jenkins pod
        nodeAffinity:
            requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
                ...
    dnspolicy: Default                              # k8s dns policy configuration for jenkins pod
    nodeselector:                                   # k8s node selector configuration for jenkins pod
        disktype: ssd
    nodename: kube-01                               # k8s node name configuration for jenkins pod
    tolerations:                                    # k8s tolerations config for jenkins pod
        - key: "key"
          operator: "Equal"
          value: "value"
          effect: "NoSchedule"
```

## Managing Jenkins resources

Jenkins resources can be managed through `cascconfig` stanza. For example, you could create a `ConfigMap` with the following [JCASC](https://github.com/jenkinsci/configuration-as-code-plugin) DSL:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: jenkins-config
data:
  jenkins-config: |
    jobs:
      - script: >
           pipelineJob('casc-job') {
             definition {
               cps {
                 script("""\
                   pipeline {
                     agent any
                     stages {
                       stage ('test') {
                         steps {
                           echo "hello"
                         }
                       }
                     }
                   }""".stripIndent())
               }
             }
           }
    jenkins:
      agentProtocols:
        - "JNLP4-connect"
        - "Ping"
      authorizationStrategy:
        loggedInUsersCanDoAnything:
          allowAnonymousRead: false
      crumbIssuer:
        standard:
          excludeClientIPFromCrumb: false
      disableRememberMe: false
      mode: NORMAL
      numExecutors: 1
      primaryView:
        all:
          name: "all"
      quietPeriod: 5
      scmCheckoutRetryCount: 3
      securityRealm:
        local:
          allowsSignup: false
          enableCaptcha: false
          users:
            - id: ${JENKINS_ADMIN_USER}
              password: ${JENKINS_ADMIN_PASSWORD}
      slaveAgentPort: 50000
      systemMessage: "jenkins-operator managed Jenkins instance\n\n"
    security:
      remotingCLI:
        enabled: false
    unclassified:
      location:
        adminAddress: admin@domain.com
        url: https://jenkins.domain.com
    tool:
      git:
        installations:
          - home: "git"
            name: "Default"
      jdk:
        defaultProperties:
          - installSource:
              installers:
                - jdkInstaller:
                    acceptLicense: false
    credentials:
      system:
        domainCredentials:
        - credentials:
          - usernamePassword:
              scope:    GLOBAL
              id:       casc-username-password
              username: root
              password: ${SUDO_PASSWORD}
```

and a secret to do substitutions for `${SUDO_PASSWORD}`, `${JENKINS_ADMIN_USER}` and `${JENKINS_ADMIN_PASSWORD}`:

```
---
apiVersion: v1
kind: Secret
metadata:
  name: jenkins-admin-secret
data:
  JENKINS_ADMIN_PASSWORD: cGFzc3dvcmQ=
  SUDO_PASSWORD: cGFzc3dvcmQ=
  JENKINS_ADMIN_USER: YWRtaW4=
type: Opaque
```

and then use them in a `JenkinsInstance` custom resource:

```
apiVersion: jenkins.jenkinsoperator.samsung-cnct.github.com/v1alpha2
kind: JenkinsInstance
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: jenkinsinstance-sample
spec:
  image: "jenkins/jenkins:lts"
  env:
    SOME_ENV: "test"
  plugins:
    - id: greenballs
      version: latest
  cascconfig:
    configmap: jenkins-config
  cascsecret: jenkins-admin-secret
  executors: 5
  adminsecret: jenkins-admin-secret
  service:
    name: jenkins
    servicetype: NodePort
  storage:
    jobspvc: jenkins
    jobspvcspec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 1G


```

This would spin up a Jenkins instance with `admin`/`password` user, a single pipeline job named `casc-job` and a `casc-username-password` credential.  
Complete JCASC documentation is available at https://github.com/jenkinsci/configuration-as-code-plugin

### Plugin management

Although JCASC does allow for plugin management from DSL, plugins that require Jenkins restart (like `kubernetes` plugin for example) should be installed prior to Jenkins startup.  
use the CR `plugins` stanza for those.

After the installation everything can be configured with the JCASC DSL.

### Non-JCASC configuration

If you need to configure something usupported by JCASC, you can use the CR `groovysecret` stanza. This allows you to specify a secret name, containing groovy script data that will be mounted into `${JENKINS_HOME}/init.groovy.d` and launched prior to Jenkins startup, but after plugin installation.

For example:

```
apiVersion: v1
kind: Secret
metadata:
  name: jenkins-groovy-secret
type: Opaque
data:
  master-agent-security: aW1wb3J0IGplbmtpbnMubW9kZWwuSmVua2lucwppbXBvcnQgamVua2lucy5zZWN1cml0eS5zMm0uKgoKSmVua2lucyBqZW5raW5zID0gSmVua2lucy5nZXRJbnN0YW5jZSgpCgovLyBFbmFibGUgQWdlbnQgdG8gbWFzdGVyIHNlY3VyaXR5IHN1YnN5c3RlbQpqZW5raW5zLmluamVjdG9yLmdldEluc3RhbmNlKEFkbWluV2hpdGVsaXN0UnVsZS5jbGFzcykuc2V0TWFzdGVyS2lsbFN3aXRjaChmYWxzZSk7CgpqZW5raW5zLnNhdmUoKQ==
```

encodes the following groovy script in the `master-agent-security` key:

```
import jenkins.model.Jenkins
import jenkins.security.s2m.*

Jenkins jenkins = Jenkins.getInstance()

// Enable Agent to master security subsystem
jenkins.injector.getInstance(AdminWhitelistRule.class).setMasterKillSwitch(false);

jenkins.save()
```

You can use this secret in jenkins instance CR:

```
apiVersion: jenkins.jenkinsoperator.samsung-cnct.github.com/v1alpha2
kind: JenkinsInstance
...
spec:
  ...
  groovysecret: jenkins-groovy-secret
  ...
```

The resulting Jenkins instance will have a `master-agent-security.groovy` file in `/var/jenkins_home/init.groovy.d/master-agent-security.groovy` that will launch during Jenkins startup and enable Jenkins's agent-to-master security subsystem.

# Build

## Install tools

* [Install](https://book.kubebuilder.io/getting_started/installation_and_setup.html) kubebuilder
* Clone this repository into `$GOPATH/src/github.com/samsung-cnct/jenkins-operator`
* Install other tool dependencies by running `make -f build/Makefile install-dep`
* Build for your OS with
  * `make -f build/Makefile darwin`
  * `make -f build/Makefile linux`
  * `make -f build/Makefile container`
