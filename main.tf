data "aws_iam_role" "lambda" {
  name = "GuardDuty2Slack"
}

resource "null_resource" "build" {
  triggers {
    main   = "${base64sha256(file("${path.module}/main.yml"))}"
    config = "${base64sha256(file("${path.module}/main.go"))}"
    make   = "${base64sha256(file("${path.module}/Makefile"))}"
  }

  provisioner "local-exec" { 
    command = "cd ${path.module} && make -f ${path.module}/Makefile"
  }
}

# https://github.com/terraform-providers/terraform-provider-archive/issues/11
data "null_data_source" "wait-for-build" {
  inputs = {
    build = "${null_resource.build.id}"
    dir   = "${path.module}/build"
  }
}

data "archive_file" "lambda" {
  type        = "zip"
  output_path = "${path.module}/main.zip"
  source_dir  = "${data.null_data_source.wait-for-build.outputs["dir"]}"
}

resource "aws_lambda_function" "lambda" {
  function_name = "GuardDuty2Slack"

  filename         = "${data.archive_file.lambda.output_path}"
  source_code_hash = "${data.archive_file.lambda.output_base64sha256}"
  handler          = "main"
  runtime          = "go1.x"

  role = "${data.aws_iam_role.lambda.arn}"

  tags {
    Name        = "GuardDuty2Slack"
    Environment = "Prod"
    Department  = "Engineering"
    Team        = "Cloud"
    Product     = "Cloud"
    Service     = "Guard Duty"
    Owner       = "cloud@my.domain"
  }
}

resource "aws_cloudwatch_event_rule" "lambda" {
  name          = "GuardDuty2Slack"
  description   = "AWS GuardDuty Finding Events"
  event_pattern = "${file("${path.module}/event-pattern.json")}"

  tags {
    Name        = "GuardDuty2Slack"
    Environment = "Prod"
    Department  = "Engineering"
    Team        = "Cloud"
    Product     = "Cloud"
    Service     = "Guard Duty"
    Owner       = "cloud@my.domain"
  }
}

resource "aws_cloudwatch_event_target" "lambda" {
  target_id = "GuardDuty2Slack"
  rule      = "${aws_cloudwatch_event_rule.lambda.name}"
  arn       = "${aws_lambda_function.lambda.arn}"
}

resource "aws_lambda_permission" "lambda" {
  statement_id  = "GuardDuty2Slack-cloudwatch-event-rule"
  action        = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.lambda.function_name}"
  principal     = "events.amazonaws.com"
  source_arn    = "${aws_cloudwatch_event_rule.lambda.arn}"
}
