# JenkinsCI operator
[![Build Status](https://jenkins.migrations.cnct.io/buildStatus/icon?job=jenkins-operator/master)](https://jenkins.migrations.cnct.io/job/jenkins-operator/job/master/)

## Project status: alpha

Major planned features have been completed, and while no breaking API changes are currently planned, we reserve the right to address bugs and API changes in a backwards incompatible way before the project is declared stable.

We expect to consider the jenkins operator stable soon; backwards incompatible changes will not be made once the project reaches stability.

## Overview

The jenkins operator manages JenkinsCI instances deployed to [Kubernetes][k8s-home] and automates tasks related to operating a JenkinsCI server

- [Create and Destroy instances](#create-and-destroy-instances)
- [Create and Destroy Jobs and Credentials](#create-and-destroy-jobs-and-credentials)

## Requirements

- Kubernetes 1.9+
- Jenkins 2.1+

## Getting started

### Deploy jenkins operator

```bash
$ helm install deployments/helm/jenkins-operator --name jenkins-operator
```

### Create a secret with Jenkins admin credentials
```bash
$ kubectl create -f config/samples/jenkinssecret.yaml 
```

A Kubernetes secret called `jenkins-test` with `user` = `admin` and `pass` = `password` will be created

### Create a Jenkins instance
```bash
$ kubectl create -f config/samples/jenkins_v1alpha1_jenkinsinstance.yaml
```

This will create a Jenkins Kubernetes deployment along with a `NodePort` Kubernetes service, a 1Gb Kubernetes persistent volume for job storage, as well as an ingress and a few other Kubernetes objects.
This instance will have a few default plugins pre-installed, along with a `greenballs` plugin at version `1.15`.

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

### Create a job with some bound credentials
```bash
$ kubectl create -f config/samples/jenkins_v1alpha1_jenkinsjob.yaml
```

This will create a couple of jobs in the JenkinsInstance you have created above. `jenkinsjob-sample-dsl` and `jenkinsjob-sample-xml`.  
`JenkinsJob` Custom resource allows you to specify jobs via either XML or [JobDSL][job-dsl]

This will also create a Kubernetes secret called `jenkins-credentials`. Data from this secret will be mounted into Jenkins as [Jenkins Credentials][jenkins-creds]  
Bindings between secret data fields and Jenkins credentails are specified in the `credentials:` section of `jenkinsjob-sample-dsl` JenkinsJob

## Configuration
## Create and Destroy instances

Instance creation and destruction is controlled through `JenkinsInstance` Kubernetes custom resources:

```
apiVersion: jenkins.jenkinsoperator.maratoid.github.com/v1alpha1
kind: JenkinsInstance
metadata:
    labels:                                         # dictionary of Kubernetes labels
        stuff.cnct.io: "woo"
    name: jenkinsinstance-sample                    # Name for this Jenkins instance object
spec:
    image: "jenkins/jenkins:lts"                    # docker image with tag for this jenkins deployment
    env:                                            # dictionary of environment variables to be set in the jenkins instance
        SOME_ENV: "test"
    plugins:                                        # dictionary of Jenkins plugins to install
        - id: greenballs                            # plugin id
          version: "1.15"                           # plugin version
    annotations:                                    # Jenkins deployment annotations
        cnct.io/annotation: "test"
        cnct.io/other-annotation: "other test"
    executors: 2                                    # Number of Jenkins executors to setup
    adminsecret: jenkins-test                       # name of the pre-existing kubernetes secret with admin user credentials
    location: https://jenkins.example.com           # Jenkins location URL
    adminemail: admin@example.com                   # Jenkins admin email
    service:                                        # Kubernetes service options. If not present, default service is created
        name: jenkins                               # service name
        servicetype: NodePort                       # service type
        annotations:
          cnct.io/service-annotation: "test"        # service annotations
    ingress:                                        # Kubernetes ingress options. If not present, no Ingress will be created
        annotations:                                # ingress annotations
          cnct.io/ingress-annotation: "test"
        service: jenkins                            # ingress service. if not specified, the default service name is used
        path: /                                     # ingress path
    serviceaccount:                                 # Kubernetes service account options. If not specified, no service account is created
        name: jenkins                               # Service account name
    networkpolicy: true                             # If true, netwrok policy will be created 
    storage:                                        # storage options
        jobspvc: jenkins                            # Name of PVC for job storage. If does not exist it will be created. If name is not specified, EmptyDir is used
        jobspvcspec:                                # If PVC is to be created, use this spec
          accessModes:                              # PVC access modes
            - ReadWriteOnce
          resources:                                # PVC requests
            requests:
              storage: 1Gi
    rbac:                                           # RBAC settings
        clusterrole: jenkins                        # name of pre-existing ClusterRole to bind service account to
    config: |                                       # Groovy configuration script string to run on startup of jenkins 
        import hudson.*
        import hudson.model.*
        import jenkins.model.*
        import jenkins.model.Jenkins.*
        import org.jenkinsci.plugins.configfiles.*
        import org.jenkinsci.lib.configprovider.model.*
        import org.jenkinsci.plugins.configfiles.groovy.*
        import org.jenkinsci.plugins.configfiles.*
        import org.jenkinsci.plugins.scriptsecurity.scripts.*
        import org.jenkinsci.plugins.scriptsecurity.scripts.languages.*
        
        GlobalConfigFiles globalConfigFiles = Jenkins.getInstance().getExtensionList(GlobalConfigFiles.class).get(GlobalConfigFiles.class);
        ConfigFileStore store = globalConfigFiles.get();
        
        
        
        String defaultJenkinsfile = """import io.cnct.pipeline.*
        new cnctPipeline().execute()"""
        
        ApprovalContext approvalContext = ApprovalContext.create();
        String defaultJenkinsfileHash = new ScriptApproval.PendingScript(
          defaultJenkinsfile, 
          GroovyLanguage.get(), 
          approvalContext).getHash();
        
        Config config = new GroovyScript(
          'Jenkinsfile', 
          'Jenkinsfile', 
          'Default pipeline Jenkinsfile', 
          defaultJenkinsfile,
          'org.jenkinsci.plugins.configfiles.groovy.GroovyScript');
        store.save(config);
        
        ScriptApproval.get().approveScript(defaultJenkinsfileHash) 
```

## Create and Destroy Jobs and Credentials

Jenkins job and credetial creation and destruction is controlled through `JenkinsJob` Kubernetes custom resources:

```
apiVersion: jenkins.jenkinsoperator.maratoid.github.com/v1alpha1
kind: JenkinsJob
metadata:
  labels:                                               # dictionary of Kubernetes labels
    stuff.cnct.io: "woo"
  name: jenkinsjob-sample-dsl                           # name of this JenkinsJob object
spec:
  jenkinsinstance: jenkinsinstance-sample               # name of the pre-existing JenkinsInstance object
  jobdsl: |                                             # Jenkins JobDSL string to use for job creation. Don't specify both this and jobxml
      freeStyleJob('jenkinsjob-sample-dsl') {

          description('With JobDSL')
          displayName('From custom resource DSL')

          steps {
              shell('echo Hello World!')
          }
      }
  jobxml: |                                             # Jenkins XML string to use for job creation. Don't specify both this and jobdsl
      <?xml version="1.0" encoding="UTF-8"?><project>
          <actions/>
          <description>From XML</description>
          <keepDependencies>false</keepDependencies>
          <properties/>
          <scm class="hudson.scm.NullSCM"/>
          <canRoam>true</canRoam>
          <disabled>false</disabled>
          <blockBuildWhenDownstreamBuilding>false</blockBuildWhenDownstreamBuilding>
          <blockBuildWhenUpstreamBuilding>false</blockBuildWhenUpstreamBuilding>
          <triggers/>
          <concurrentBuild>false</concurrentBuild>
          <builders>
              <hudson.tasks.Shell>
                  <command>echo Hello World with xml!</command>
              </hudson.tasks.Shell>
          </builders>
          <publishers/>
          <buildWrappers/>
          <displayName>From custom resource XML</displayName>
      </project>
  credentials:                                          # bindings of Kubernetes secrets to Jenkins credentials                      
    - credential: userpass-creds                        # Jenkins credential name
      secret: jenkins-credentials                       # Kubernetes secret name
      credentialtype: usernamePassword                  # type of Jenkins credential. Can be usernamePassword|secretText|serviceaccount|vaultgithub|vaultapprole|vaulttoken
      secretdata:                                       # Jenkins credential field to Kubernetes data field mapping.
        username: username-data
        password: password-data
```

### Credential types

Supported `credentialtype` 's are: `usernamePassword|secretText|serviceaccount|vaultgithub|vaultapprole|vaulttoken`

|Credential type|Supported fields|
|--:|---|
|`serviceaccount`|N/A|
|`secretText`|`secret`|
|`usernamePassword`|`username`,`password`|
|`vaultgithub`|`accessToken`|
|`vaultapprole`|`roleId`,`secretId`|
|`vaulttoken`|`token`|

### Additional credential types

Additional credential types can be bound by using the [kubernetes-credentials-provider-plugin][k8screds]

[k8s-home]: http://kubernetes.io
[job-dsl]: https://github.com/jenkinsci/job-dsl-plugin
[jenkins-creds]: https://jenkins.io/doc/book/using/using-credentials/
[k8screds]:https://jenkinsci.github.io/kubernetes-credentials-provider-plugin/