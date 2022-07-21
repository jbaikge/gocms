import { RemovalPolicy, Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as apigw from 'aws-cdk-lib/aws-apigateway';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import { join } from 'path';
import { Bucket, BucketAccessControl } from 'aws-cdk-lib/aws-s3';
import { BucketDeployment, Source } from 'aws-cdk-lib/aws-s3-deployment';
import { Distribution, OriginAccessIdentity } from 'aws-cdk-lib/aws-cloudfront';
import { S3Origin } from 'aws-cdk-lib/aws-cloudfront-origins';


export class GocmsStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    // DynamoDB table
    const classTable = new dynamodb.Table(this, 'Classes', {
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      // Not used when billingMode is PAY_PER_REQUEST
      // readCapacity: 1,
      // writeCapacity: 1,
      removalPolicy: RemovalPolicy.DESTROY,
      partitionKey: {
        name: 'ClassId',
        type: dynamodb.AttributeType.STRING
      },
    });

    // Asset directory where all the lambda binaries come from
    const assetDir = join(__dirname, '..', '..', 'assets');

    // Create class lambda function
    const createClassLambda = new lambda.Function(this, 'CreateClassHandler', {
      environment: {
        'DYNAMODB_CLASS_TABLE': classTable.tableName,
      },
      runtime: lambda.Runtime.GO_1_X,
      code:    lambda.Code.fromAsset(join(assetDir, 'create-class')),
      handler: 'handler'
    });
    classTable.grantWriteData(createClassLambda);

    // Get class lambda function
    const getClassByIdLambda = new lambda.Function(this, 'GetClassByIdHandler', {
      environment: {
        'DYNAMODB_CLASS_TABLE': classTable.tableName,
      },
      runtime: lambda.Runtime.GO_1_X,
      code:    lambda.Code.fromAsset(join(assetDir, 'get-class-by-id')),
      handler: 'handler'
    });
    classTable.grantReadData(getClassByIdLambda);

    // List class lambda function
    const listClassesLambda = new lambda.Function(this, 'ListClassesHandler', {
      environment: {
        'DYNAMODB_CLASS_TABLE': classTable.tableName,
      },
      runtime: lambda.Runtime.GO_1_X,
      code:    lambda.Code.fromAsset(join(assetDir, 'list-classes')),
      handler: 'handler'
    });
    classTable.grantReadData(listClassesLambda);

    // Update class lambda function
    const updateClassLambda = new lambda.Function(this, 'UpdateClassHandler', {
      environment: {
        'DYNAMODB_CLASS_TABLE': classTable.tableName,
      },
      runtime: lambda.Runtime.GO_1_X,
      code:    lambda.Code.fromAsset(join(assetDir, 'update-class')),
      handler: 'handler'
    });
    classTable.grantWriteData(updateClassLambda);

    // REST API
    const api = new apigw.RestApi(this, 'GoCMS Endpoint', {
      defaultCorsPreflightOptions: {
        allowOrigins: apigw.Cors.ALL_ORIGINS,
        allowHeaders: [
          'Content-Type',
          'Range',
          'Authorization'
        ],
        exposeHeaders: [
          'Content-Range',
        ],
      }
    });

    // Class endpoints
    const classResource = api.root.addResource('classes');

    // Allow the Range header with requests for pagination
    // Ref: https://rahullokurte.com/how-to-validate-requests-to-the-aws-api-gateway-using-cdk
    // Ref: https://stackoverflow.com/a/68305757
    const listClassesIntegration = new apigw.LambdaIntegration(listClassesLambda, {
      requestParameters: {
        "integration.request.header.range": "method.request.header.range",
      },
    });
    classResource.addMethod('GET', listClassesIntegration, {
      requestParameters: {
        "method.request.header.range": false,
      },
    });

    classResource.addMethod('POST', new apigw.LambdaIntegration(createClassLambda));

    const classIdResource = classResource.addResource('{id}')
    classIdResource.addMethod('GET', new apigw.LambdaIntegration(getClassByIdLambda));
    classIdResource.addMethod('PUT', new apigw.LambdaIntegration(updateClassLambda));

    // Admin frontend
    // https://aws-cdk.com/deploying-a-static-website-using-s3-and-cloudfront/
    const adminBucket = new Bucket(this, 'FrontendAdminBucket', {
      accessControl: BucketAccessControl.PRIVATE,
    });

    new BucketDeployment(this, 'FrontendAdminBucketDeployment', {
      destinationBucket: adminBucket,
      sources: [Source.asset(join(__dirname, '..', '..', '_frontend-admin', 'build'))],
    });

    const adminOriginAccessIdentity = new OriginAccessIdentity(this, 'FrontendAdminOAI');
    adminBucket.grantRead(adminOriginAccessIdentity);

    new Distribution(this, 'FrontendAdminDistribution', {
      defaultRootObject: 'index.html',
      defaultBehavior: {
        origin: new S3Origin(adminBucket, {originAccessIdentity: adminOriginAccessIdentity}),
      }
    });
  }
}
