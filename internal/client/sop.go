package client

import (
	"context"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"net/url"
)

func sopPath(pid, did id.ID) string { return divisionPath(pid, did) + "/sop" }
func (c *Client) SOPScopes(ctx context.Context, pid id.ID) ([]api.SOPAccess, error) {
	var out api.List[api.SOPAccess]
	err := c.do(ctx, "GET", "/api/periods/"+pid.String()+"/sop-scopes", nil, &out)
	return out.Items, err
}
func (c *Client) SOPPolicy(ctx context.Context, pid, did id.ID) (api.SOPPolicy, error) {
	var out api.SOPPolicy
	err := c.do(ctx, "GET", sopPath(pid, did)+"/policy", nil, &out)
	return out, err
}
func (c *Client) SetSOPPolicy(ctx context.Context, pid, did id.ID, req api.SOPPolicyRequest) (api.SOPPolicy, error) {
	var out api.SOPPolicy
	err := c.do(ctx, "PUT", sopPath(pid, did)+"/policy", req, &out)
	return out, err
}
func (c *Client) SOPPeople(ctx context.Context, pid, did id.ID) ([]api.SOPPerson, error) {
	var out api.List[api.SOPPerson]
	err := c.do(ctx, "GET", sopPath(pid, did)+"/people", nil, &out)
	return out.Items, err
}
func (c *Client) SOPContents(ctx context.Context, pid, did id.ID) ([]api.SOPContentRef, error) {
	var out api.List[api.SOPContentRef]
	err := c.do(ctx, "GET", sopPath(pid, did)+"/contents", nil, &out)
	return out.Items, err
}
func (c *Client) SOPContext(ctx context.Context, pid, did, cid id.ID) (api.SOPContext, error) {
	var out api.SOPContext
	err := c.do(ctx, "GET", sopPath(pid, did)+"/contexts/"+cid.String(), nil, &out)
	return out, err
}
func (c *Client) SetSOPContext(ctx context.Context, pid, did, cid id.ID, req api.SOPContextRequest) (api.SOPContext, error) {
	var out api.SOPContext
	err := c.do(ctx, "PUT", sopPath(pid, did)+"/contexts/"+cid.String(), req, &out)
	return out, err
}
func (c *Client) SOPFindings(ctx context.Context, pid, did id.ID, filter url.Values) ([]api.SOPFinding, error) {
	page, err := c.SOPFindingPage(ctx, pid, did, filter)
	return page.Items, err
}
func (c *Client) SOPFindingPage(ctx context.Context, pid, did id.ID, filter url.Values) (api.SOPFindingPage, error) {
	var out api.SOPFindingPage
	path := sopPath(pid, did) + "/findings"
	if len(filter) > 0 {
		path += "?" + filter.Encode()
	}
	err := c.do(ctx, "GET", path, nil, &out)
	return out, err
}
func (c *Client) SOPFinding(ctx context.Context, pid, did, fid id.ID) (api.SOPFinding, error) {
	var out api.SOPFinding
	err := c.do(ctx, "GET", sopPath(pid, did)+"/findings/"+fid.String(), nil, &out)
	return out, err
}

func (c *Client) SOPFindingHistory(ctx context.Context, pid, did, fid id.ID) (api.SOPHistory, error) {
	var out api.SOPHistory
	err := c.doAudit(ctx, "GET", sopPath(pid, did)+"/findings/"+fid.String()+"/history", nil, &out)
	return out, err
}
func (c *Client) DecideSOPFinding(ctx context.Context, pid, did, fid id.ID, req api.SOPDecisionRequest) (api.SOPFinding, error) {
	var out api.SOPFinding
	err := c.do(ctx, "POST", sopPath(pid, did)+"/findings/"+fid.String()+"/decision", req, &out)
	return out, err
}
func (c *Client) ManualSOPFinding(ctx context.Context, pid, did id.ID, req api.SOPManualFindingRequest) (id.ID, error) {
	var out struct {
		ID id.ID `json:"id"`
	}
	err := c.do(ctx, "POST", sopPath(pid, did)+"/findings", req, &out)
	return out.ID, err
}
func (c *Client) SOPReport(ctx context.Context, pid, did id.ID, filter url.Values) (api.SOPReport, error) {
	var out api.SOPReport
	query := url.Values{}
	for key, values := range filter {
		query[key] = append([]string{}, values...)
	}
	query.Set("summaryOnly", "true")
	err := c.do(ctx, "GET", sopPath(pid, did)+"/report?"+query.Encode(), nil, &out)
	return out, err
}
func (c *Client) SOPEvaluatorCandidates(ctx context.Context, pid, did id.ID) ([]api.SOPEvaluatorCandidate, error) {
	var out api.List[api.SOPEvaluatorCandidate]
	err := c.do(ctx, "GET", sopPath(pid, did)+"/evaluator-candidates", nil, &out)
	return out.Items, err
}
func (c *Client) SOPEvaluatorGrants(ctx context.Context, pid, did id.ID) ([]api.SOPEvaluatorGrant, error) {
	var out api.List[api.SOPEvaluatorGrant]
	err := c.do(ctx, "GET", sopPath(pid, did)+"/evaluators", nil, &out)
	return out.Items, err
}
func (c *Client) GrantSOPEvaluator(ctx context.Context, pid, did id.ID, req api.SOPEvaluatorGrantRequest) error {
	return c.do(ctx, "POST", sopPath(pid, did)+"/evaluators", req, nil)
}
func (c *Client) RevokeSOPEvaluator(ctx context.Context, pid, did, gid id.ID, reason string) error {
	return c.do(ctx, "POST", sopPath(pid, did)+"/evaluators/"+gid.String()+"/revoke", map[string]string{"reason": reason}, nil)
}
func (c *Client) SOPBotAgreement(ctx context.Context, pid, did id.ID, approved bool, version int64, reason string) error {
	return c.do(ctx, "POST", sopPath(pid, did)+"/bot-agreement", map[string]any{"approved": approved, "expectedVersion": version, "reason": reason}, nil)
}

func (c *Client) SOPDeliveries(ctx context.Context, pid, did id.ID, cursor string) (api.SOPDeliveryPage, error) {
	var out api.SOPDeliveryPage
	path := sopPath(pid, did) + "/deliveries"
	if cursor != "" {
		path += "?" + url.Values{"cursor": {cursor}}.Encode()
	}
	err := c.do(ctx, "GET", path, nil, &out)
	return out, err
}
func (c *Client) SOPDelivery(ctx context.Context, pid, did, rid id.ID) (api.SOPDelivery, error) {
	var out api.SOPDelivery
	err := c.doAudit(ctx, "GET", sopPath(pid, did)+"/deliveries/"+rid.String(), nil, &out)
	return out, err
}

func (c *Client) SOPTransferPreview(ctx context.Context, pid, did, fid, targetDivision id.ID, rule string) (api.SOPTransferPreview, error) {
	var out api.SOPTransferPreview
	query := url.Values{"divisionId": {targetDivision.String()}, "rule": {rule}}
	err := c.do(ctx, "GET", sopPath(pid, did)+"/findings/"+fid.String()+"/transfer-preview?"+query.Encode(), nil, &out)
	return out, err
}
func (c *Client) TransferSOPFinding(ctx context.Context, pid, did, fid id.ID, req api.SOPTransferRequest) (api.SOPTransfer, error) {
	var out api.SOPTransfer
	err := c.do(ctx, "POST", sopPath(pid, did)+"/findings/"+fid.String()+"/transfer", req, &out)
	return out, err
}

func (c *Client) SOPReportCSV(ctx context.Context, pid, did id.ID, filter url.Values) ([]byte, error) {
	query := url.Values{}
	for key, values := range filter {
		query[key] = append([]string{}, values...)
	}
	query.Set("format", "csv")
	query.Del("summaryOnly")
	return c.download(ctx, sopPath(pid, did)+"/report?"+query.Encode(), "text/csv", 32<<20)
}

func (c *Client) SOPReadiness(ctx context.Context, pid, did id.ID, cid *id.ID) (api.SOPReadiness, error) {
	var out api.SOPReadiness
	path := sopPath(pid, did) + "/schedule-readiness"
	if cid != nil {
		path += "?" + url.Values{"contentId": {cid.String()}}.Encode()
	}
	err := c.do(ctx, "GET", path, nil, &out)
	return out, err
}

// SOPSummary is the Evaluasi SOP chart of a quarter (?quarter=) or range
// (?from=&to=), per rule (Tahap 7a).
func (c *Client) SOPSummary(ctx context.Context, pid, did id.ID, filter url.Values) (api.SOPSummary, error) {
	var out api.SOPSummary
	err := c.do(ctx, "GET", sopPath(pid, did)+"/summary?"+filter.Encode(), nil, &out)
	return out, err
}
