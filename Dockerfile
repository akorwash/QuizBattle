version: '1.0'
steps:
  main_clone:
    title: Cloning main repository...
    type: git-clone
    repo: 'github.com/akorwash/QuizBattle'
    revision: master
    git: github
  MyAppDockerImage:
    title: Building Docker Image
    type: build
    image_name: my-golang-image
    working_directory: ./src/
    tag: full
    dockerfile: Dockerfile