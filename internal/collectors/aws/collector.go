package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Collector fetches live AWS resource state
type Collector struct {
	cfg    *config.Config
	db     *pgxpool.Pool
	engine *engine.Engine
	logger *zap.Logger
}

// NewCollector creates an AWS collector
func NewCollector(cfg *config.Config, db *pgxpool.Pool, eng *engine.Engine, logger *zap.Logger) *Collector {
	return &Collector{
		cfg:    cfg,
		db:     db,
		engine: eng,
		logger: logger,
	}
}

// Collect scans all AWS resources for all orgs with active AWS credentials
func (c *Collector) Collect(ctx context.Context) error {
	c.logger.Info("starting AWS collection")
	start := time.Now()

	// Load all orgs with active AWS credentials
	rows, err := c.db.Query(ctx, `
		SELECT cc.id, cc.org_id, cc.credentials
		FROM cloud_credentials cc
		WHERE cc.provider = 'aws' AND cc.is_active = true
	`)
	if err != nil {
		return fmt.Errorf("querying aws credentials: %w", err)
	}
	defer rows.Close()

	type credRow struct {
		ID          uuid.UUID
		OrgID       uuid.UUID
		Credentials json.RawMessage
	}

	var creds []credRow
	for rows.Next() {
		var cr credRow
		if err := rows.Scan(&cr.ID, &cr.OrgID, &cr.Credentials); err != nil {
			return fmt.Errorf("scanning credential row: %w", err)
		}
		creds = append(creds, cr)
	}

	if len(creds) == 0 {
		// Fallback: use env-based credentials if configured
		if c.cfg.AWSAccessKeyID != "" {
			c.logger.Info("no DB credentials found, using env-based AWS credentials")
			return c.collectForOrg(ctx, uuid.Nil, c.cfg.AWSAccessKeyID, c.cfg.AWSSecretAccessKey, c.cfg.AWSRegion)
		}
		c.logger.Info("no AWS credentials configured, skipping collection")
		return nil
	}

	for _, cr := range creds {
		var awsCreds struct {
			AccessKeyID     string `json:"access_key_id"`
			SecretAccessKey string `json:"secret_access_key"`
			Region          string `json:"region"`
		}
		if err := json.Unmarshal(cr.Credentials, &awsCreds); err != nil {
			c.logger.Error("failed to parse AWS credentials", zap.String("org_id", cr.OrgID.String()), zap.Error(err))
			continue
		}
		if err := c.collectForOrg(ctx, cr.OrgID, awsCreds.AccessKeyID, awsCreds.SecretAccessKey, awsCreds.Region); err != nil {
			c.logger.Error("collection failed for org", zap.String("org_id", cr.OrgID.String()), zap.Error(err))
		}
	}

	c.logger.Info("AWS collection complete", zap.Duration("duration", time.Since(start)))
	return nil
}

// collectForOrg runs collection for a single org's AWS account
func (c *Collector) collectForOrg(ctx context.Context, orgID uuid.UUID, accessKeyID, secretKey, region string) error {
	// Build AWS config
	var cfg aws.Config
	var err error

	if accessKeyID != "" && secretKey != "" {
		cfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(region),
			awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, ""),
			),
		)
	} else {
		// Use default credential chain (IAM role, instance profile, env vars)
		cfg, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	}
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	var resources []*models.Resource

	// Collect EC2 instances
	ec2Resources, err := c.collectEC2(ctx, cfg, orgID, region)
	if err != nil {
		c.logger.Warn("EC2 collection failed", zap.Error(err), zap.String("org_id", orgID.String()))
	} else {
		resources = append(resources, ec2Resources...)
	}

	// Collect S3 buckets
	s3Resources, err := c.collectS3(ctx, cfg, orgID, region)
	if err != nil {
		c.logger.Warn("S3 collection failed", zap.Error(err), zap.String("org_id", orgID.String()))
	} else {
		resources = append(resources, s3Resources...)
	}

	// Collect IAM roles (always use us-east-1 for IAM)
	iamRegion := "us-east-1"
	iamCfg, iamErr := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(iamRegion))
	if iamErr != nil {
		c.logger.Warn("IAM config load failed", zap.Error(iamErr), zap.String("org_id", orgID.String()))
	} else {
		iamResources, err := c.collectIAM(ctx, iamCfg, orgID)
		if err != nil {
			c.logger.Warn("IAM collection failed", zap.Error(err), zap.String("org_id", orgID.String()))
		} else {
			resources = append(resources, iamResources...)
		}
	}

	c.logger.Info("collected AWS resources",
		zap.Int("count", len(resources)),
		zap.String("org_id", orgID.String()),
	)

	// Upsert resources and detect drift
	for _, r := range resources {
		if err := c.engine.UpsertResource(ctx, r); err != nil {
			c.logger.Error("failed to upsert resource", zap.String("resource_id", r.ResourceID), zap.Error(err))
		}
	}

	return nil
}

// collectEC2 fetches all EC2 instances and returns them as Resource objects
func (c *Collector) collectEC2(ctx context.Context, cfg aws.Config, orgID uuid.UUID, region string) ([]*models.Resource, error) {
	client := ec2.NewFromConfig(cfg)

	var resources []*models.Resource
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running", "stopped", "stopping"},
			},
		},
	})

	now := time.Now().UTC()
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			c.logger.Warn("EC2 pagination failed", zap.Error(err), zap.String("org_id", orgID.String()))
			return nil, fmt.Errorf("describing EC2 instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				// Extract tags as a map
				tags := make(map[string]string)
				name := ""
				for _, tag := range instance.Tags {
					tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
					if aws.ToString(tag.Key) == "Name" {
						name = aws.ToString(tag.Value)
					}
				}

				iamProfile := iamProfileName(instance.IamInstanceProfile)
				liveState := map[string]interface{}{
					"instance_id":            aws.ToString(instance.InstanceId),
					"instance_type":          string(instance.InstanceType),
					"state":                  string(instance.State.Name),
					"availability_zone":      aws.ToString(instance.Placement.AvailabilityZone),
					"subnet_id":              aws.ToString(instance.SubnetId),
					"vpc_id":                 aws.ToString(instance.VpcId),
					"private_ip":             aws.ToString(instance.PrivateIpAddress),
					"public_ip":              aws.ToString(instance.PublicIpAddress),
					"ami_id":                 aws.ToString(instance.ImageId),
					"key_name":               aws.ToString(instance.KeyName),
					"iam_profile":            iamProfile,
					"monitoring":             string(instance.Monitoring.State),
					"ebs_optimized":          aws.ToBool(instance.EbsOptimized),
					"termination_protection": false, // fetched separately if needed
					"tags":                   tags,
					"security_groups":        extractSecurityGroupIDs(instance.SecurityGroups),
					// "scanned_at":  now, // Do not include in drift input
				}

				stateBytes, _ := json.Marshal(liveState)

				r := &models.Resource{
					OrgID:         orgID,
					Provider:      "aws",
					Region:        region,
					ResourceType:  "ec2_instance",
					ResourceID:    aws.ToString(instance.InstanceId),
					ResourceName:  name,
					LiveState:     stateBytes,
					LastScannedAt: now,
				}
				resources = append(resources, r)
			}
		}
	}

	return resources, nil
}

// collectS3 fetches all S3 buckets
func (c *Collector) collectS3(ctx context.Context, cfg aws.Config, orgID uuid.UUID, region string) ([]*models.Resource, error) {
	client := s3.NewFromConfig(cfg)

	result, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("listing S3 buckets: %w", err)
	}

	var resources []*models.Resource
	now := time.Now().UTC()
	for _, bucket := range result.Buckets {
		bucketName := aws.ToString(bucket.Name)

		// Get bucket versioning
		versioningStatus := "Disabled"
		versioning, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
			Bucket: aws.String(bucketName),
		})
		if err == nil && versioning.Status != "" {
			versioningStatus = string(versioning.Status)
		}

		// Get bucket encryption
		encryptionEnabled := false
		_, err = client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
			Bucket: aws.String(bucketName),
		})
		if err == nil {
			encryptionEnabled = true
		}

		// Get public access block
		publicAccessBlocked := false
		pab, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
			Bucket: aws.String(bucketName),
		})
		if err == nil && pab.PublicAccessBlockConfiguration != nil {
			cfg := pab.PublicAccessBlockConfiguration
			publicAccessBlocked = aws.ToBool(cfg.BlockPublicAcls) &&
				aws.ToBool(cfg.BlockPublicPolicy) &&
				aws.ToBool(cfg.IgnorePublicAcls) &&
				aws.ToBool(cfg.RestrictPublicBuckets)
		}

		liveState := map[string]interface{}{
			"bucket_name":           bucketName,
			"creation_date":         bucket.CreationDate,
			"versioning_status":     versioningStatus,
			"encryption_enabled":    encryptionEnabled,
			"public_access_blocked": publicAccessBlocked,
			"region":                region,
			// "scanned_at":         now, // Do not include in drift input
		}

		stateBytes, _ := json.Marshal(liveState)

		resources = append(resources, &models.Resource{
			OrgID:         orgID,
			Provider:      "aws",
			Region:        region,
			ResourceType:  "s3_bucket",
			ResourceID:    bucketName,
			ResourceName:  bucketName,
			LiveState:     stateBytes,
			LastScannedAt: now,
		})
	}

	return resources, nil
}

// collectIAM fetches IAM roles
func (c *Collector) collectIAM(ctx context.Context, cfg aws.Config, orgID uuid.UUID) ([]*models.Resource, error) {
	client := iam.NewFromConfig(cfg)

	var resources []*models.Resource
	paginator := iam.NewListRolesPaginator(client, &iam.ListRolesInput{})

	now := time.Now().UTC()
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			c.logger.Warn("IAM pagination failed", zap.Error(err), zap.String("org_id", orgID.String()))
			return nil, fmt.Errorf("listing IAM roles: %w", err)
		}

		for _, role := range page.Roles {
			liveState := map[string]interface{}{
				"role_name":            aws.ToString(role.RoleName),
				"role_id":              aws.ToString(role.RoleId),
				"arn":                  aws.ToString(role.Arn),
				"description":          aws.ToString(role.Description),
				"max_session_duration": role.MaxSessionDuration,
				"assume_role_policy":   aws.ToString(role.AssumeRolePolicyDocument),
				"path":                 aws.ToString(role.Path),
				"created_at":           role.CreateDate,
				// "scanned_at":        now, // Do not include in drift input
			}

			stateBytes, _ := json.Marshal(liveState)

			resources = append(resources, &models.Resource{
				OrgID:         orgID,
				Provider:      "aws",
				Region:        "global",
				ResourceType:  "iam_role",
				ResourceID:    aws.ToString(role.RoleName),
				ResourceName:  aws.ToString(role.RoleName),
				LiveState:     stateBytes,
				LastScannedAt: now,
			})
		}
	}

	return resources, nil
}

// --- helpers ---

func iamProfileName(profile *ec2types.IamInstanceProfile) string {
	if profile == nil || profile.Arn == nil {
		return ""
	}
	return *profile.Arn
}

func extractSecurityGroupIDs(sgs []ec2types.GroupIdentifier) []string {
	result := make([]string, 0, len(sgs))
	for _, sg := range sgs {
		if sg.GroupId != nil {
			result = append(result, *sg.GroupId)
		}
	}
	return result
}
