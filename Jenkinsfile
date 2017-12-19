pipeline {
  agent {
    label "test"
  }
  options {
    buildDiscarder(logRotator(numToKeepStr: '2'))
    disableConcurrentBuilds()
  }
  stages {
    stage("build") {
      steps {
        sh "docker image build -t thomasjpfan/docker-scaler-docs -f Dockerfile.docs ."
      }
    }
    stage("release") {
      when {
        branch "master"
      }
      steps {
        withCredentials([usernamePassword(
          credentialsId: "docker_thomasjpfan",
          usernameVariable: "USER",
          passwordVariable: "PASS"
        )]) {
          sh "docker login -u $USER -p $PASS"
        }
        sh "docker image push thomasjpfan/docker-scaler-docs:latest"
      }
    }
    stage("deploy") {
      when {
        branch "master"
      }
      agent {
        label "prod"
      }
      steps {
        script {
            def psStatus = sh(
                script: "docker service ps scaler_docs",
                returnStatus: true
            )
            sh "docker service update --image thomasjpfan/docker-scaler-docs:latest scaler_docs"
        }
      }
    }
  }
  post {
    always {
      sh "docker system prune -f"
    }
  }
}
