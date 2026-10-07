package supportdesk

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) IssueVerifiedAnonymousLink(ctx context.Context, ticketNo, destinationKind, destinationDigest string) (*AnonymousAccess, error) {
	ticketNo = strings.TrimSpace(ticketNo)
	destinationKind = strings.TrimSpace(destinationKind)
	destinationDigest = strings.TrimSpace(destinationDigest)
	if !businessNoPattern.MatchString(ticketNo) || (destinationKind != "email" && destinationKind != "sms") || len(destinationDigest) != sha256.Size*2 {
		return nil, bizInvalid()
	}
	if _, err := hex.DecodeString(destinationDigest); err != nil || destinationDigest != strings.ToLower(destinationDigest) {
		return nil, bizInvalid()
	}
	linkNo, err := s.newNo("SAL_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate anonymous support link number")
	}
	raw := make([]byte, 32)
	if _, err = io.ReadFull(s.random, raw); err != nil {
		return nil, wrapUnavailable(err, "generate anonymous support access token")
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := anonymousTokenDigest(token)
	var result *AnonymousAccess
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		count, loadErr := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no=? AND ticket_type='presales' AND visibility_code='anonymous' AND anonymous_destination_kind=? AND anonymous_destination_digest=?`, ticketNo, destinationKind, destinationDigest)
		if loadErr != nil {
			return loadErr
		}
		if count != 1 {
			return bizNotAccessible()
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		expires := anonymousAccessExpiry(now)
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_access_link (tenant_id,link_no,ticket_no,token_digest,destination_digest,destination_kind,expires_at,created_at) VALUES (0,?,?,?,?,?,?,?)`, linkNo, ticketNo, digest, destinationDigest, destinationKind, expires, now); insertErr != nil {
			return insertErr
		}
		result = &AnonymousAccess{LinkNo: linkNo, Token: token, TicketNo: ticketNo, ExpiresAt: expires}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, wrapUnavailable(err, "issue anonymous support access")
	}
	return result, nil
}

func (s *serviceImpl) DetailAnonymous(ctx context.Context, token string) (*Detail, error) {
	ticket, err := s.anonymousTicket(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return s.anonymousDetail(ctx, ticket)
}

func (s *serviceImpl) ReplyAnonymous(ctx context.Context, token string, input MessageInput) (*Detail, error) {
	body, ok := normalizeText(input.Body, maxTicketBodyBytes)
	if !ok || input.TicketVersion <= 0 || len(input.UploadNos) > 20 {
		return nil, bizInvalid()
	}
	messageNo, err := s.newNo("MSG_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate anonymous support reply")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.anonymousTicket(ctx, token, true)
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion || ticket.BaseStage == StageClosed {
			return bizerr.NewCode(CodeConflict)
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		stage := ticket.BaseStage
		if stage == "waiting_customer" || stage == "resolved" {
			stage = "in_progress"
		}
		updated, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage=?,customer_last_replied_at=?,last_activity_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, stage, now, now, now, ticket.TicketNo, input.TicketVersion)
		if updateErr != nil {
			return updateErr
		}
		changed, rowsErr := updated.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) VALUES (0,?,?, 'public_reply','customer_visible','anonymous',0,?,FALSE,?)`, messageNo, ticket.TicketNo, body, now); insertErr != nil {
			return insertErr
		}
		if consumeErr := s.consumeUploads(ctx, ticket.TicketNo, messageNo, input.UploadNos, 0, "customer_visible", "anonymous_reply", now); consumeErr != nil {
			return consumeErr
		}
		if stage != ticket.BaseStage {
			if historyErr := s.appendStatusHistory(ctx, ticket.TicketNo, ticket.BaseStage, stage, "anonymous", 0, "anonymous_reply", "", now); historyErr != nil {
				return historyErr
			}
			if ticket.BaseStage == "waiting_customer" {
				return s.finishPause(ctx, ticket, now)
			}
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, wrapUnavailable(err, "reply to anonymous support ticket")
	}
	return s.DetailAnonymous(ctx, token)
}

func (s *serviceImpl) CloseAnonymous(ctx context.Context, token string, ticketVersion int64) (*Detail, error) {
	if err := s.transitionAnonymous(ctx, token, ticketVersion, StageClosed, "anonymous_confirmed"); err != nil {
		return nil, err
	}
	return s.DetailAnonymous(ctx, token)
}

func (s *serviceImpl) ReopenAnonymous(ctx context.Context, token string, ticketVersion int64) (*Detail, error) {
	if err := s.transitionAnonymous(ctx, token, ticketVersion, "in_progress", "anonymous_reopen"); err != nil {
		return nil, err
	}
	return s.DetailAnonymous(ctx, token)
}

func (s *serviceImpl) RateAnonymous(ctx context.Context, token string, input RatingInput) (*Detail, error) {
	feedback := strings.TrimSpace(input.Feedback)
	if input.Stars < 1 || input.Stars > 5 || input.RoundNo <= 0 || input.TicketVersion <= 0 || len([]byte(feedback)) > 2000 {
		return nil, bizInvalid()
	}
	if input.Stars <= 3 && (s.staffAccess == nil || s.notifications == nil) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	ratingNo, err := s.newNo("RAT_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate anonymous ticket rating")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, err := s.anonymousTicket(ctx, token, true)
		if err != nil {
			return err
		}
		if ticket.Version != input.TicketVersion || input.RoundNo != ticket.CurrentRound || (ticket.BaseStage != StageResolved && ticket.BaseStage != StageClosed) {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		if ticket.ClosedAt != nil && now.Sub(*ticket.ClosedAt) > 7*24*time.Hour {
			return bizerr.NewCode(CodeConflict)
		}
		existing, err := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_rating WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, ticket.TicketNo, ticket.CurrentRound)
		if err != nil {
			return err
		}
		if existing != 0 {
			return bizerr.NewCode(CodeConflict)
		}
		recipients := []int64{}
		if input.Stars <= 3 {
			recipients, err = s.eligibleRatingRecipients(ctx, ticket)
			if err != nil {
				return err
			}
			if len(recipients) == 0 {
				return bizerr.NewCode(CodeUnavailable)
			}
			if input.Stars == 3 {
				recipients = onlyRecipient(recipients, ticket.AssigneeUserID)
				if len(recipients) == 0 {
					return bizerr.NewCode(CodeUnavailable)
				}
			}
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_rating (tenant_id,rating_no,ticket_no,round_no,score,comment,author_type,created_at) VALUES (0,?,?,?,?,?,'anonymous',?)`, ratingNo, ticket.TicketNo, input.RoundNo, input.Stars, nullable(feedback), now); err != nil {
			return err
		}
		if input.Stars <= 2 {
			result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET follow_up_required=TRUE,version=version+1,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, ticket.TicketNo, ticket.Version)
			if updateErr != nil {
				return updateErr
			}
			if changed, _ := result.RowsAffected(); changed != 1 {
				return bizerr.NewCode(CodeConflict)
			}
			for _, recipient := range recipients {
				if eventErr := s.enqueueStaffTicketEvent(ctx, ticket, supportEventTicketRatingLow, recipient, ticket.Version+1, map[string]string{"rating": strconv.Itoa(input.Stars)}); eventErr != nil {
					return eventErr
				}
			}
		} else if input.Stars == 3 {
			for _, recipient := range recipients {
				if eventErr := s.enqueueStaffTicketEvent(ctx, ticket, supportEventTicketRatingNeutral, recipient, ticket.Version, map[string]string{"rating": strconv.Itoa(input.Stars)}); eventErr != nil {
					return eventErr
				}
			}
		}
		return nil
	})
	if err = anonymousWriteResult(err, "rate anonymous support ticket"); err != nil {
		return nil, err
	}
	return s.DetailAnonymous(ctx, token)
}

func (s *serviceImpl) transitionAnonymous(ctx context.Context, token string, ticketVersion int64, toStage, reasonCode string) error {
	if ticketVersion <= 0 || (toStage != StageClosed && toStage != StageInProgress) {
		return bizInvalid()
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, err := s.anonymousTicket(ctx, token, true)
		if err != nil {
			return err
		}
		if ticket.Version != ticketVersion || (toStage == StageClosed && ticket.BaseStage == StageClosed) || (toStage == StageInProgress && ticket.BaseStage != StageClosed) {
			return bizerr.NewCode(CodeConflict)
		}
		if toStage == StageInProgress {
			allowed, err := s.withinReopenWindow(ctx, ticket)
			if err != nil {
				return err
			}
			if !allowed {
				return bizerr.NewCode(CodeConflict)
			}
			if ticket.AssigneeUserID != 0 {
				allowed, allowErr := s.targetStaffAllowed(ctx, ticket.AssigneeUserID, staffCapabilityHandle, ticket.DepartmentNo, false)
				if allowErr != nil {
					return allowErr
				}
				if !allowed {
					ticket.AssigneeUserID = 0
				}
			}
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		newRound := ticket.CurrentRound
		if toStage == StageInProgress {
			newRound++
		}
		var result sql.Result
		if toStage == StageClosed {
			result, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage='closed',sla_risk_code=CASE WHEN sla_risk_code='at_risk' THEN 'none' ELSE sla_risk_code END,closed_at=?,closed_reason_code=?,closed_by_type='anonymous',last_activity_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, reasonCode, now, now, ticket.TicketNo, ticket.Version)
		} else {
			result, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage='in_progress',current_round=?,assignee_user_id=?,sla_risk_code='none',resolved_at=NULL,closed_at=NULL,closed_reason_code=NULL,closed_by_type=NULL,last_activity_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, newRound, nullableInt(ticket.AssigneeUserID), now, now, ticket.TicketNo, ticket.Version)
		}
		if err = requireOneChanged(result, err); err != nil {
			return err
		}
		if err = s.appendStatusHistory(ctx, ticket.TicketNo, ticket.BaseStage, toStage, "anonymous", 0, reasonCode, "", now); err != nil {
			return err
		}
		if toStage == StageClosed {
			_, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET resolved_at=COALESCE(resolved_at,?),stopped_at=? WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, now, now, ticket.TicketNo, ticket.CurrentRound)
			return err
		}
		return s.createSLARound(ctx, ticket.TicketNo, newRound, ticket.DepartmentNo, ticket.CategoryNo, ticket.Priority, now)
	})
	return anonymousWriteResult(err, "transition anonymous support ticket")
}

func (s *serviceImpl) anonymousTicket(ctx context.Context, token string, lock bool) (*Ticket, error) {
	token = strings.TrimSpace(token)
	if len(token) < 40 || len(token) > 128 {
		return nil, bizNotAccessible()
	}
	query := `SELECT ` + ticketProjection("t", "c", "customer") + `,l.expires_at AS anonymous_access_expires_at FROM plugin_idc_core_support_ticket_access_link l JOIN plugin_idc_core_support_ticket t ON t.tenant_id=0 AND t.ticket_no=l.ticket_no LEFT JOIN plugin_idc_core_support_issue_category c ON c.tenant_id=0 AND c.category_no=t.category_no LEFT JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=0 AND customer.customer_no=t.customer_no WHERE l.tenant_id=0 AND l.token_digest=? AND l.revoked_at IS NULL AND l.expires_at>CURRENT_TIMESTAMP AND t.ticket_type='presales' AND t.visibility_code='anonymous'`
	if lock {
		query += ` FOR UPDATE OF l,t`
	} else {
		query += ` LIMIT 1`
	}
	rows, err := scanTickets(ctx, s.database, query, anonymousTokenDigest(token))
	if err != nil {
		return nil, wrapUnavailable(err, "verify anonymous support access")
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return rows[0], nil
}

func (s *serviceImpl) anonymousDetail(ctx context.Context, ticket *Ticket) (*Detail, error) {
	var messages []Message
	if err := s.database.GetScan(ctx, &messages, `SELECT message_no,message_type AS type,visibility_code AS visibility,author_type,0 AS author_user_id,body,automated,FALSE AS hidden,created_at FROM plugin_idc_core_support_ticket_message WHERE tenant_id=0 AND ticket_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL ORDER BY created_at,id`, ticket.TicketNo); err != nil {
		return nil, wrapUnavailable(err, "load anonymous support messages")
	}
	attachments, err := s.loadAttachments(ctx, ticket.TicketNo, false)
	if err != nil {
		return nil, err
	}
	for i := range messages {
		messages[i].Attachments = attachments[messages[i].MessageNo]
	}
	customFields, err := s.loadCustomFieldValues(ctx, ticket.TicketNo, false)
	if err != nil {
		return nil, err
	}
	detail := &Detail{Ticket: ticket, Messages: messages, CustomFields: customFields, AnonymousAccessExpiresAt: ticket.AnonymousAccessExpiresAt}
	if err = s.loadCustomerActionState(ctx, detail); err != nil {
		return nil, err
	}
	return detail, nil
}

func anonymousWriteResult(err error, operation string) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return wrapUnavailable(err, operation)
}

func anonymousTokenDigest(token string) string {
	digest := sha256.Sum256([]byte("idc-support-anonymous-access-v1\x00" + token))
	return hex.EncodeToString(digest[:])
}

func anonymousAccessExpiry(signedAt time.Time) time.Time {
	return signedAt.Add(anonymousAccessLifetime)
}
