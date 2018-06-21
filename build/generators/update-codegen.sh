#!/usr/bin/env bash

# Setting some variables up here
THIS_DIRECTORY="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PACKAGE_HOME=${THIS_DIRECTORY}/../../
PACKAGE_NAME=github.com/maratoid/jenkins-operator
PACKAGE_VIRTUAL=/go/src/${PACKAGE_NAME}
API_PACKAGE=jenkinsoperator/v1alpha1
GENERATED_CODE_HOME=pkg/generated/jenkinsoperator/client

echo "Removing old generated code"
rm -rf ${PACKAGE_HOME}/${GENERATED_CODE_HOME}
mkdir -p ${PACKAGE_HOME}/${GENERATED_CODE_HOME}

# Creating the deep copy object
echo "Creating the deep copy object in ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE} ... "
deepcopy-gen --input-dirs ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE} -h ${THIS_DIRECTORY}/custom-boilerplate.go.txt
printf ".... done creating deep copy object\n\n"

# Creating the openapi (validation) meta information
echo "Creating the openapi validation object in ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE} ... "
openapi-gen -i ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE},k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/api/core/v1 \
       -p ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE} -h ${THIS_DIRECTORY}/custom-boilerplate.go.txt
printf ".... done creating openapi validation object\n\n"

# Creating the clientset
echo "Creating the clientset in ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/clientset ... "
client-gen -p ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/clientset --input-base ${PACKAGE_NAME}/pkg/apis --input $API_PACKAGE -n versioned \
       -h ${THIS_DIRECTORY}/custom-boilerplate.go.txt
printf ".... done creating the clientset\n\n"

# Creating the lister
echo "Creating the lister in ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/listers ... "
lister-gen -p ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/listers --input-dirs ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE} \
       -h ${THIS_DIRECTORY}/custom-boilerplate.go.txt
printf ".... done creating the lister\n\n"

# Creating the informer
echo "Creating the informer in ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/informers ... "
informer-gen -p ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/informers --input-dirs ${PACKAGE_NAME}/pkg/apis/${API_PACKAGE} \
       --versioned-clientset-package ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/clientset/versioned --listers-package ${PACKAGE_NAME}/${GENERATED_CODE_HOME}/listers \
       -h ${THIS_DIRECTORY}/custom-boilerplate.go.txt
printf ".... done creating the informer\n\n"