/*
 * Mini Copy (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package s3

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/minio-io/mc/pkg/client"
	"github.com/minio-io/minio/pkg/iodine"
)

/// Object Operations PUT - keeping this in a separate file for readability

// Put - upload new object to bucket
func (c *s3Client) Put(md5HexString string, size int64) (io.WriteCloser, error) {
	bucket, object := c.url2BucketAndObject()
	if !client.IsValidBucketName(bucket) || strings.Contains(bucket, ".") {
		return nil, iodine.New(InvalidBucketName{Bucket: bucket}, nil)
	}
	r, w := io.Pipe()
	blockingWriter := client.NewBlockingWriteCloser(w)
	go func() {
		if size < 0 {
			err := iodine.New(client.InvalidArgument{Err: errors.New("invalid argument")}, nil)
			r.CloseWithError(err)
			blockingWriter.Release(err)
			return
		}
		req, err := c.newRequest("PUT", c.objectURL(bucket, object), r)
		if err != nil {
			err := iodine.New(err, nil)
			r.CloseWithError(err)
			blockingWriter.Release(err)
			return
		}
		req.Method = "PUT"
		req.ContentLength = size

		// set Content-MD5 only if md5 is provided
		if strings.TrimSpace(md5HexString) != "" {
			md5, err := hex.DecodeString(md5HexString)
			if err != nil {
				err := iodine.New(err, nil)
				r.CloseWithError(err)
				blockingWriter.Release(err)
				return
			}
			req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(md5))
		}
		if c.AccessKeyID != "" && c.SecretAccessKey != "" {
			c.signRequest(req, c.Host)
		}
		// this is necessary for debug, since the underlying transport is a wrapper
		res, err := c.Transport.RoundTrip(req)
		if err != nil {
			err := iodine.New(err, nil)
			r.CloseWithError(err)
			blockingWriter.Release(err)
			return
		}
		if res.StatusCode != http.StatusOK {
			err := iodine.New(NewError(res), nil)
			r.CloseWithError(err)
			blockingWriter.Release(err)
			return
		}
		r.Close()
		blockingWriter.Release(nil)
	}()
	return blockingWriter, nil
}
