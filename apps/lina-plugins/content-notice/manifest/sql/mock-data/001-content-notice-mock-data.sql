-- 幂等公告演示数据：只含标题、正文图片、状态、排序与基本操作者和时间。
INSERT INTO plugin_content_notice (tenant_id,title,content,status,sort_order,created_by,updated_by,created_at,updated_at)
SELECT 0,sample.title,sample.content,sample.status,sample.sort_order,admin.id,admin.id,sample.created_at::TIMESTAMPTZ,sample.created_at::TIMESTAMPTZ
FROM (VALUES
    ('系统升级公告','<p>系统将于本周六凌晨2:00-4:00进行升级维护，请提前做好安排。</p>',1,0,'2026-04-20 09:00:00+08'),
    ('关于规范使用系统的公告','<p>为保障系统安全稳定运行，请定期修改密码，不要将账号密码告知他人。</p>',1,1,'2026-04-21 10:30:00+08'),
    ('新功能上线预告','<p>我们即将上线公告管理、公告阅读和富文本编辑器。</p>',0,2,'2026-04-22 14:15:00+08'),
    ('安全巡检结果公告','<p>本周安全巡检已完成，核心服务、插件运行时和数据库备份链路均正常。</p>',1,3,'2026-04-23 16:45:00+08')
) AS sample(title,content,status,sort_order,created_at)
CROSS JOIN sys_user admin
WHERE admin.username='admin'
AND NOT EXISTS (SELECT 1 FROM plugin_content_notice existing WHERE existing.tenant_id=0 AND existing.title=sample.title AND existing.created_by=admin.id AND existing.created_at=sample.created_at::TIMESTAMPTZ);
