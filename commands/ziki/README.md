# Ziki: The Search for Merge

A Wikimedia adventure in 16 assignments: from first patch to incident commander.
Start with `mw ziki`. Use `--level 13` to try the expert round directly.
Nothing connects to real services; progress and input history last for this game.

## How to play

Type a destination to start its activity, then choose a numbered response.
The next-task hint tells you where the required work is. Some assignments revisit
the same destination several times: investigation, a fix, review, retesting, a
canary, rollback and another canary are separate tasks.

Tasks must be completed in order. Visiting Production as a new contributor does
not give you deployment responsibilities. Your first assignment ends at merge;
someone else deploys it. Later assignments grant wider scope and responsibility.

Type `next` after completing an assignment. `levels` shows the campaign.
`work` retries the current activity. `look` and `status` only redraw the scene.
`help` lists commands. Wrong decisions cost morale; AFK restores it. Exhaustion
ends the game. The correct answer is not always the first option.

Destinations and commands remain on screen. Names accept unambiguous prefixes
and aliases, ignoring case: `cli`, `phabricator`, `review`, `tg`, `dc`, `prod`,
`bp`, `tools`, `vps`, `docs`, `graphs`, `logs`, `demo`, `media`, `i18n`, `beta`,
`search`, `calendar`, `adminlog`. `go <destination>` works too.

Interactive terminals clear on each turn, keeping the last result visible.
`--no-clear` keeps a transcript, as does redirected output. Up and Down recall
previous inputs. `--color none` disables colours; `color dark|light|none` works
inside the game. `quit` works at every prompt.

## Assignments

The main campaign covers a basic patch, a translated feature, Toolforge, Commons,
Cloud VPS recovery, backports, supervised deployment, scraper mitigation,
cross-component changes, staging differences, the release train and overlapping
community-service failures.

Four expert assignments follow: a translated regression, a failed canary,
a blocked train and a multi-cause incident. They require 14–27 ordered tasks,
including returning to logs, reviews, branch tests and deployment checks.

## AI assistance

Type `ai` before your patch is merged. Reviewing catches bad suggestions; blind
trust can send your assignment back to a validation checkpoint. Successful AI
assistance never bypasses the required tasks. Every paid attempt upgrades the
MergeMind model, doubles token usage and quadruples its bill. Failed attempts
are charged too. The accumulated bill survives between levels.

## Credits

The name combines "Zork" and "Wiki". The original game was inspired by a
comment from [Cindy Cicalese](https://meta.wikimedia.org/wiki/User:CCicalese_(WMF)).
Initial implementation was by [Bill Pirkle](https://meta.wikimedia.org/wiki/User:BPirkle_(WMF)).
Implementation ideas and some original code came from
[Paul Fortin's blog](https://gocodecloud.com/blog/2016/03/19/writing-a-text-adventure-game-in-go---part-1/).

This is affectionate fiction, not an operational runbook.
