from openai import OpenAI

client = OpenAI(api_key="OPENAI_API_KEY")

# Read knowledge base
with open("knowledge.txt") as file:
    context = file.read()

# Read questions
with open("question.txt") as file:
    questions = file.read()

SYSTEM_PROMPT = """
You are an expert Security and Compliance AI assistant for ACME Corp. 
    You will be provided with the company's internal security policy and a list of questions from a client RFP.
    
    Rule 1: You must answer the questions using ONLY the provided security policy.
    Rule 2: If the answer to a question is not explicitly stated in the policy, you must reply: "Information not available in the provided context."
    Rule 3: Keep your answers professional, concise, and directly address the question.
"""

USER_PROMPT = f"""
    --- COMPANY SECURITY POLICY ---
    {context}
    
    --- RFP QUESTIONS ---
    {questions}
"""

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "system", "content": SYSTEM_PROMPT},
              {"role": "user", "content": USER_PROMPT}],
    temperature=0.1
)

print(response.choices[0].message.content)
