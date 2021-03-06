package gocms

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/zeebo/assert"
)

var regionFlag string

func init() {
	flag.StringVar(&regionFlag, "region", "local", "DynamoDB region: local for dy compatibility; localhost for nosql workbench compatibility")
}

func TestDynamoDBClassConversion(t *testing.T) {
	classNilFields := &Class{
		Id:   "NilFields",
		Name: "Nil Fields",
	}

	dynamoNilFields := new(dynamoClass)
	dynamoNilFields.FromClass(classNilFields)

	jsonDynamoNilFields, err := json.Marshal(dynamoNilFields)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(jsonDynamoNilFields), `"Fields":[]`))

	classFromDynamo := dynamoNilFields.ToClass()
	jsonClassNilFields, err := json.Marshal(classFromDynamo)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(jsonClassNilFields), `"fields":[]`))
}

func TestDynamoDBDocumentConversion(t *testing.T) {
	doc := &Document{
		Id:    "TestDoc",
		Title: "Test Doc",
		Url:   "/test/doc",
		Values: map[string]interface{}{
			"date": "2022-07-28",
		},
	}

	dynamoDoc := dynamoDocument{}
	dynamoDoc.FromDocument(doc)

	jsonDynamoDoc, err := json.Marshal(dynamoDoc)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(jsonDynamoDoc), `"ParentId":"#NULL#"`))

	fromDynamo := dynamoDoc.ToDocument()
	jsonDoc, err := json.Marshal(fromDynamo)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(jsonDoc), `"parent_id":""`))
}

func TestDynamoDBRepositoryClass(t *testing.T) {
	resources := DynamoDBResources{
		Tables: DynamoDBTables{
			Class: t.Name() + time.Now().Format("-20060102-150405"),
		},
	}

	endpointResolverFunc := func(service string, region string, options ...interface{}) (endpoint aws.Endpoint, err error) {
		endpoint = aws.Endpoint{
			PartitionID:   "aws",
			URL:           "http://localhost:8000",
			SigningRegion: regionFlag,
		}
		return
	}

	endpointResolver := aws.EndpointResolverWithOptionsFunc(endpointResolverFunc)

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithEndpointResolverWithOptions(endpointResolver),
	)
	assert.NoError(t, err)

	// TODO Move this block elsewhere, maybe into the makefile or somewhere it
	// can match what is described in the deployment stack.
	client := dynamodb.NewFromConfig(cfg)
	_, err = client.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: &resources.Tables.Class,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("ClassId"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("ClassId"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	assert.NoError(t, err)

	repo := NewDynamoDBRepository(cfg, resources)

	// Use of UnixMicro trims off the m variable in the Time struct to make
	// reflect.DeepEqual function properly
	now := time.UnixMicro(time.Now().UnixMicro())
	class1 := Class{
		Id:      "1",
		Name:    "My Class",
		Created: now,
		Updated: now,
		Fields:  []Field{},
	}

	t.Run("CreateClass", func(t *testing.T) {
		assert.NoError(t, repo.CreateClass(context.Background(), &class1))
	})

	t.Run("GetClassById", func(t *testing.T) {
		check, err := repo.GetClassById(context.Background(), class1.Id)
		assert.NoError(t, err)
		assert.DeepEqual(t, class1, check)
	})

	t.Run("UpdateClass", func(t *testing.T) {
		class1.Name = "My New Class"
		assert.NoError(t, repo.UpdateClass(context.Background(), &class1))
		check, err := repo.GetClassById(context.Background(), class1.Id)
		assert.NoError(t, err)
		assert.DeepEqual(t, class1, check)
	})

	t.Run("GetClassList", func(t *testing.T) {
		count := 10
		for i := 2; i <= count; i++ {
			class := Class{
				Id:      fmt.Sprintf("%d", i),
				Name:    fmt.Sprintf("Class %d", i),
				Created: now,
				Updated: now,
			}
			assert.NoError(t, repo.CreateClass(context.Background(), &class))
		}

		t.Run("All", func(t *testing.T) {
			filter := ClassFilter{
				Range: Range{
					Start: 0,
					End:   count - 1,
				},
			}

			classes, r, err := repo.GetClassList(context.Background(), filter)
			assert.NoError(t, err)
			assert.Equal(t, filter.Range.Start, r.Start)
			assert.Equal(t, filter.Range.End, r.End)
			assert.Equal(t, count, r.Size)
			assert.Equal(t, count, len(classes))
		})

		t.Run("Front", func(t *testing.T) {
			filter := ClassFilter{
				Range: Range{
					Start: 0,
					End:   4,
				},
			}

			total := filter.Range.End - filter.Range.Start + 1

			classes, r, err := repo.GetClassList(context.Background(), filter)
			assert.NoError(t, err)
			assert.Equal(t, filter.Range.Start, r.Start)
			assert.Equal(t, filter.Range.End, r.End)
			assert.Equal(t, count, r.Size)
			assert.Equal(t, total, len(classes))
		})

		t.Run("Middle", func(t *testing.T) {
			filter := ClassFilter{
				Range: Range{
					Start: 3,
					End:   6,
				},
			}

			total := filter.Range.End - filter.Range.Start + 1

			classes, r, err := repo.GetClassList(context.Background(), filter)
			assert.NoError(t, err)
			assert.Equal(t, filter.Range.Start, r.Start)
			assert.Equal(t, filter.Range.End, r.End)
			assert.Equal(t, count, r.Size)
			assert.Equal(t, total, len(classes))
		})

		t.Run("Back", func(t *testing.T) {
			filter := ClassFilter{
				Range: Range{
					Start: 5,
					End:   9,
				},
			}

			total := filter.Range.End - filter.Range.Start + 1

			classes, r, err := repo.GetClassList(context.Background(), filter)
			assert.NoError(t, err)
			assert.Equal(t, filter.Range.Start, r.Start)
			assert.Equal(t, filter.Range.End, r.End)
			assert.Equal(t, count, r.Size)
			assert.Equal(t, total, len(classes))
		})

		t.Run("Overflow", func(t *testing.T) {
			// In the API this should throw an HTTP 416 Range Not Satisfiable
			// Ref: https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/416
			filter := ClassFilter{
				Range: Range{
					Start: 15,
					End:   19,
				},
			}

			_, _, err := repo.GetClassList(context.Background(), filter)
			assert.Error(t, err)
		})

	})

	t.Run("DeleteClass", func(t *testing.T) {
		assert.NoError(t, repo.DeleteClass(context.Background(), class1.Id))
		_, err := repo.GetClassById(context.Background(), class1.Id)
		assert.Error(t, err)
	})
}

func TestDynamoDBRepositoryDocument(t *testing.T) {
	resources := DynamoDBResources{
		Tables: DynamoDBTables{
			Document: t.Name() + time.Now().Format("-20060102-150405"),
		},
	}

	endpointResolverFunc := func(service string, region string, options ...interface{}) (endpoint aws.Endpoint, err error) {
		endpoint = aws.Endpoint{
			PartitionID:   "aws",
			URL:           "http://localhost:8000",
			SigningRegion: regionFlag,
		}
		return
	}

	endpointResolver := aws.EndpointResolverWithOptionsFunc(endpointResolverFunc)

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithEndpointResolverWithOptions(endpointResolver),
	)
	assert.NoError(t, err)

	// TODO Move this block elsewhere, maybe into the makefile or somewhere it
	// can match what is described in the deployment stack.
	client := dynamodb.NewFromConfig(cfg)
	_, err = client.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: &resources.Tables.Document,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("DocumentId"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("ClassId"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("ParentId"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("Version"),
				AttributeType: types.ScalarAttributeTypeN,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("DocumentId"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("Version"),
				KeyType:       types.KeyTypeRange,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("GSI-Class"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("ClassId"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("Version"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
			{
				IndexName: aws.String("GSI-Parent"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("ParentId"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("Version"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	assert.NoError(t, err)

	repo := NewDynamoDBRepository(cfg, resources)

	t.Run("CreateDocument", func(t *testing.T) {
		doc := Document{
			Id:         "doc_1",
			ClassId:    "class_1",
			TemplateId: "template_1",
			ParentId:   "doc_0",
			Title:      "CreateDocument Test",
		}
		assert.NoError(t, repo.CreateDocument(context.Background(), &doc))
	})

	t.Run("CreateDocumentNilParent", func(t *testing.T) {
		doc := Document{
			Id:      "doc_2",
			ClassId: "class_1",
			Title:   "CreateDocument Nil Parent Test",
		}
		assert.NoError(t, repo.CreateDocument(context.Background(), &doc))
	})

	t.Run("GetDocumentById", func(t *testing.T) {
		doc := Document{
			Id:         "doc_3",
			ClassId:    "class_2",
			TemplateId: "template_2",
			ParentId:   "doc_1",
		}
		assert.NoError(t, repo.CreateDocument(context.Background(), &doc))
		check, err := repo.GetDocumentById(context.Background(), doc.Id)
		assert.NoError(t, err)
		assert.DeepEqual(t, doc, check)
	})

	t.Run("UpdateDocument", func(t *testing.T) {
		doc := Document{
			Id:         "doc_4",
			ClassId:    "class_2",
			TemplateId: "template_2",
			ParentId:   "doc_1",
		}
		assert.NoError(t, repo.CreateDocument(context.Background(), &doc))

		doc.TemplateId = "template_3"
		assert.NoError(t, repo.UpdateDocument(context.Background(), &doc))

		check, err := repo.GetDocumentById(context.Background(), doc.Id)
		assert.NoError(t, err)
		assert.DeepEqual(t, doc, check)
	})

	t.Run("DeleteDocument", func(t *testing.T) {
		doc := Document{
			Id:         "doc_5",
			ClassId:    "class_1",
			TemplateId: "template_1",
			ParentId:   "doc_1",
		}
		assert.NoError(t, repo.CreateDocument(context.Background(), &doc))
		assert.NoError(t, repo.DeleteDocument(context.Background(), doc.Id))

		// Make sure the document no longer exists
		_, err := repo.GetDocumentById(context.Background(), doc.Id)
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})
}
