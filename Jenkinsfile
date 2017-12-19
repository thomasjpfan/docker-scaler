import java.text.SimpleDateFormat

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
        script {
          def dateFormat = new SimpleDateFormat("yy.MM.dd")
          currentBuild.displayName = dateFormat.format(new Date()) + "-" + env.BUILD_NUMBER
        }
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
        sh "docker tag thomasjpfan/docker-scaler-docs thomasjpfan/docker-scaler-docs:${currentBuild.displayName}"
        sh "docker image push thomasjpfan/docker-scaler-docs:latest"
        sh "docker image push thomasjpfan/docker-scaler-docs:${currentBuild.displayName}"
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
            if (psStatus == 0) {
                sh "docker service update --image thomasjpfan/docker-scaler-docs:${currentBuild.displayName} scaler_docs"
            } else if (psStatus == 1) {
                error "service scaler_docs not found"
            } else {
                error "docker cli not found"
            }
        }
      }
    }
  }
  post {
    always {
      sh "docker system prune -f"
    }
    failure {
      slackSend(
        color: "danger",
        message: "${env.JOB_NAME} failed: ${env.RUN_DISPLAY_URL}"
      )
    }
  }
}
