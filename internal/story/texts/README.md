# 内置短篇文本版本

这些是经过来源核对的历史文本，不是生成式模型重写的原文。中文场景、人物描述和事实梗概为编者整理，位于 library.json，允许与原文观点区分。

- magi.txt：O. Henry, The Gift of the Magi, The Four Million (1906)，Project Gutenberg #2776；从该篇标题到下一篇标题前截取全文。
- leaf.txt：O. Henry, The Last Leaf, The Trimmed Lamp (1907)，Project Gutenberg #3707；从该篇标题到下一篇标题前截取全文。
- paw.txt：W. W. Jacobs, The Monkey's Paw, The Lady of the Barge (1902)，Project Gutenberg #12122；保留故事正文与历史标题，去除站点头尾。
- kong.txt：鲁迅《孔乙己》(1919)，固定版本 https://zh.wikisource.org/w/index.php?title=孔乙己&oldid=2605389 。现展示简体完整正文：通过系统繁简转换并规范镇、煮、傻、清、青、偷、污、懒、真、坛等异体字，预备统一为常见字形。没有删改情节或重新分段；对应锚点同步转换。kong-traditional.txt 仅保留为旧存档校验基线，不作为阅读入口。
- beiying.txt：朱自清《背影》完整散文，原文版本与来源见页面。
- taohuayuan.txt：《桃花源记》散文部分全文，不包括另篇《桃花源诗》。据维基文库 https://zh.wikisource.org/zh-hans/桃花源記 修订版2620894，去掉校勘旁注，简体字形与标点整理。
- quanxue.txt：《劝学》四段课文选文，范围为君子曰、青取之于蓝、吾尝终日而思、积土成山至用心躁也；不是原篇连续全本。对照 https://www.gushiwen.cn/shiwenv_9b5ed8061abe.aspx 的公版原文，未复制现代注释与译文。原篇另见 https://zh.wikisource.org/zh-hans/荀子/勸學篇 。
- mulan.txt：《木兰诗》七段简体通行全文，对照 https://www.gushiwen.cn/shiwenv_2d6b0c83a500.aspx 。仅录公版古诗，不复制现代译注；保留火伴、帖花黄等异文。作者佚名，不将编者郭茂倩当成作者。

新增三篇核对日期：2026-09-11。注释、论证步骤、详略与路线卡由本项目编写，明确标记为解读而非古人原话。不同版本的用字、标点及分段可能与教材不同。

《孔乙己》旧阅读数据仅允许从这一个已核验字形版本迁移：保留段落ID、书签、手记、历史回答、revision与原有分支；下次明确保存时写入新文本校验值。其他未知正文变更仍拒绝静默迁移。

前三份下载地址为 https://www.gutenberg.org/cache/epub/{编号}/pg{编号}.txt ，统一 CRLF 为 LF。来源页面标示历史作品为美国公有领域；对外发布仍应核对目标地区适用权利。不使用现代中文译文，不包含站点自动生成的内容简介。

library.json 的 anchors 为对应原文中的精确片段；运行时计算 UTF-8 字节位置，测试验证逐字一致。事实可综合其他段落，界面不把中文梗概冒充直接引语。

《孔乙己》的日常事件有重复性，场景次序是编者组织，不主张原文给出了精确日期。《猴爪》含丧亲和恐怖主题；《最后一片叶子》的历史医生台词不作为现代医疗建议。
