export interface ReflectionSeed {
  id: string;
  person_id: string;
  session_id?: string;
  scope: "bond" | "self_pattern" | "tension" | "principle_candidate";
  statement: string;
  source_type: "direct_expression" | "behavior_observation" | "model_inference" | "generated_hypothesis";
  status: "open" | "deferred" | "resolved" | "superseded" | "rejected";
  generated?: boolean;
  experience_modes?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface ReflectionEvidence {
  seed_id?: string;
  episode_id: string;
  state: "support" | "conflict" | "context" | "duplicate" | "stale" | "insufficient" | "superseded";
  reason_code?: string;
  read?: boolean;
  generated?: boolean;
  experience_mode?: string;
  epistemic?: string;
}

export interface ReflectionDecision {
  seed_id: string;
  action: "no_change" | "defer" | "confirm" | "revise" | "supersede" | "open_tension" | "resolve_tension" | "reject_seed";
  kind?: string;
  field?: string;
  suggested_text?: string;
  reason_summary?: string;
  proposal_id?: string;
  mutation_id?: string;
}

export interface ReflectionRun {
  id: string;
  mode: "legacy" | "observe" | "agent";
  trigger?: string;
  seed_ids?: string[];
  candidate_ids?: string[];
  read_ids?: string[];
  evidence?: ReflectionEvidence[];
  decisions?: ReflectionDecision[];
  mutation_ids?: string[];
  generated_seed_ids?: string[];
  generated_seeds?: Array<{ id: string; scope: string; statement: string; generated: boolean }>;
  generative_note?: string;
  no_change?: boolean;
  notes?: string;
  started_at?: string;
  completed_at?: string;
}
export interface ReflectionLiveState {
  phase?: "starting" | "selecting" | "candidates" | "reading" | "evidence" | "consolidating" | "generating" | "completed";
  running?: boolean;
  run?: ReflectionRun;
}
